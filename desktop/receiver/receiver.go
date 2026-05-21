package receiver

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lsariol/plop/desktop/config"
	"github.com/lsariol/plop/internal/store"
	notify "github.com/lsariol/plop/internal/toast"
)

type Receiver struct {
	cfg          config.Config
	desktopToken string
	client       *http.Client
	onDelivery   func(folder string) // called after each successful delivery; may be nil
}

func New(cfg config.Config, desktopToken string, onDelivery func(string)) *Receiver {
	return &Receiver{
		cfg:          cfg,
		desktopToken: desktopToken,
		client:       &http.Client{Timeout: 5 * time.Minute},
		onDelivery:   onDelivery,
	}
}

func (r *Receiver) Receive(pm store.PayloadReadyMsg) error {
	folder := resolveFolder(pm.Tags, r.cfg.Tags.Rules, r.cfg.Tags.DefaultFolder)
	dest := filepath.Join(folder, time.Now().Format("2006-01-02")+"_"+pm.ID[:8])
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	req, err := http.NewRequest("GET", r.cfg.ServerURL+"/payload/"+pm.ID, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.desktopToken)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("download payload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d for payload %s", resp.StatusCode, pm.ID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	for _, f := range zr.File {
		if err := extractFile(f, dest); err != nil {
			return fmt.Errorf("extract %s: %w", f.Name, err)
		}
	}

	msg := fmt.Sprintf("Received %d file(s) → %s", pm.FileCount, dest)
	log.Printf("payload %s: %s", pm.ID[:8], msg)
	if err := notify.PushNotification("Plop", msg, dest); err != nil {
		log.Printf("toast notification failed: %v", err)
	}

	if r.onDelivery != nil {
		r.onDelivery(dest)
	}

	return nil
}

func extractFile(f *zip.File, dest string) error {
	// filepath.Base prevents path traversal (e.g. "../../../etc/passwd")
	target := filepath.Join(dest, filepath.Base(f.Name))

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func resolveFolder(tags []string, rules []config.TagRule, def string) string {
	for _, rule := range rules {
		for _, tag := range tags {
			if strings.EqualFold(strings.TrimSpace(tag), strings.TrimSpace(rule.Tag)) {
				return rule.Folder
			}
		}
	}
	return def
}
