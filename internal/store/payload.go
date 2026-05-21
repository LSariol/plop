package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// UploadedFile represents a single file coming in from a multipart upload.
// Reader is the raw stream — SavePayload writes it to disk and then closes it.
type UploadedFile struct {
	Name     string
	MimeType string
	Reader   io.Reader
}

// File is the metadata stored in Postgres after the file has been written to disk.
type File struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// Payload is the full payload model, matching the payloads table.
type Payload struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
	UserID    string
	Text      string
	Tags      []string
	Files     []File
	Acked     bool
}

type PendingMsg struct {
	Payloads []PayloadReadyMsg `json:"payloads"`
}

type AckMsg struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type PayloadReadyMsg struct {
	ID        string   `json:"id"`
	Text      string   `json:"text,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	FileCount int      `json:"file_count"`
}

// sanitizeName strips directory components from a filename using cross-platform
// logic: backslashes are normalized to slashes before path.Base is applied, so
// Windows-style paths like "..\evil" are safe on Linux servers too.
func sanitizeName(raw string) string {
	name := path.Base(strings.ReplaceAll(raw, "\\", "/"))
	if name == "." || name == "" || name == "/" {
		return "upload"
	}
	return name
}

func (s *Store) SavePayload(ctx context.Context, p Payload, uploads []UploadedFile) error {
	dir := filepath.Join(s.payloadDir, p.ID, "files")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create payload dir: %w", err)
	}

	var files []File

	// usedNames tracks filenames already written to detect and resolve collisions.
	usedNames := make(map[string]struct{})

	// Write message text as message.txt so it lands in the desktop ZIP
	if p.Text != "" {
		data := []byte(p.Text)
		if err := os.WriteFile(filepath.Join(dir, "message.txt"), data, 0640); err != nil {
			os.RemoveAll(filepath.Join(s.payloadDir, p.ID))
			return fmt.Errorf("write message file: %w", err)
		}
		usedNames["message.txt"] = struct{}{}
		files = append(files, File{
			Name:     "message.txt",
			MimeType: "text/plain; charset=utf-8",
			Size:     int64(len(data)),
		})
	}

	// Write each uploaded file to disk, collecting metadata as we go
	for _, u := range uploads {
		name := sanitizeName(u.Name)
		// Deduplicate: if name is already taken append " (n)" before the extension.
		if _, taken := usedNames[name]; taken {
			ext := filepath.Ext(name)
			base := name[:len(name)-len(ext)]
			for i := 1; ; i++ {
				candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
				if _, taken := usedNames[candidate]; !taken {
					name = candidate
					break
				}
			}
		}
		usedNames[name] = struct{}{}

		dst := filepath.Join(dir, name)
		n, err := writeFile(dst, u.Reader)
		if err != nil {
			os.RemoveAll(filepath.Join(s.payloadDir, p.ID))
			return fmt.Errorf("write file %s: %w", name, err)
		}
		files = append(files, File{
			Name:     name,
			MimeType: u.MimeType,
			Size:     n,
		})
	}
	p.Files = files

	// tags is NOT NULL in the schema — coerce nil to an empty array
	if p.Tags == nil {
		p.Tags = []string{}
	}

	filesJSON, err := json.Marshal(p.Files)
	if err != nil {
		return fmt.Errorf("marshal files: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO payloads (id, expires_at, text, tags, files, user_id)
         VALUES ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.ExpiresAt, p.Text, p.Tags, filesJSON, p.UserID,
	)
	if err != nil {
		os.RemoveAll(filepath.Join(s.payloadDir, p.ID))
		return fmt.Errorf("insert payload: %w", err)
	}
	return nil
}

func (s *Store) GetPayload(ctx context.Context, id, userID string) (Payload, error) {
	var p Payload
	var filesJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, created_at, expires_at, text, tags, files, acked
         FROM payloads WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(&p.ID, &p.CreatedAt, &p.ExpiresAt, &p.Text, &p.Tags, &filesJSON, &p.Acked)
	if errors.Is(err, pgx.ErrNoRows) {
		return Payload{}, ErrNotFound
	}
	if err != nil {
		return Payload{}, fmt.Errorf("query payload: %w", err)
	}
	if err := json.Unmarshal(filesJSON, &p.Files); err != nil {
		return Payload{}, fmt.Errorf("unmarshal files: %w", err)
	}
	return p, nil
}

func (s *Store) AckPayload(ctx context.Context, id, userID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM payloads WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete payload row: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	// Best-effort disk cleanup — don't fail the ack if removal fails.
	if removeErr := os.RemoveAll(filepath.Join(s.payloadDir, id)); removeErr != nil {
		slog.Warn("delete payload dir", "id", id, "error", removeErr)
	}
	return nil
}

func (s *Store) PendingPayloads(ctx context.Context, userID string) ([]PayloadReadyMsg, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, text, tags, files FROM payloads
         WHERE acked = false AND expires_at > now() AND user_id = $1
         ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending payloads: %w", err)
	}
	defer rows.Close()

	var result []PayloadReadyMsg
	for rows.Next() {
		var (
			id        string
			text      string
			tags      []string
			filesJSON []byte
		)
		if err := rows.Scan(&id, &text, &tags, &filesJSON); err != nil {
			return nil, fmt.Errorf("scan pending payload: %w", err)
		}
		var files []File
		if err := json.Unmarshal(filesJSON, &files); err != nil {
			return nil, fmt.Errorf("unmarshal files: %w", err)
		}
		result = append(result, PayloadReadyMsg{
			ID:        id,
			Text:      text,
			Tags:      tags,
			FileCount: len(files),
		})
	}
	return result, rows.Err()
}

func (s *Store) DeleteExpiredPayloads(ctx context.Context) error {
	rows, err := s.pool.Query(ctx,
		`DELETE FROM payloads
         WHERE expires_at < now() AND acked = false
         RETURNING id`,
	)
	if err != nil {
		return fmt.Errorf("delete expired payloads: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan expired id: %w", err)
		}
		if err := os.RemoveAll(filepath.Join(s.payloadDir, id)); err != nil {
			slog.Warn("failed to remove expired payload dir", "id", id, "error", err)
			// log and continue — don't let one bad directory block the rest
		}
	}
	return rows.Err()
}

// writeFile streams an io.Reader to disk
func writeFile(dst string, r io.Reader) (int64, error) {
	f, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}
