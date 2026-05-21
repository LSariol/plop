package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/lsariol/plop/web"
)

// computeWebVersion hashes all embedded web assets and returns the first 8
// hex characters. Called once at startup — the result is stable for a given
// binary and changes automatically whenever any web file is modified.
func computeWebVersion() string {
	h := sha256.New()
	fs.WalkDir(web.Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, _ := fs.ReadFile(web.Files, path)
		h.Write(data)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:8]
}

// ServeServiceWorker serves sw.js with __ASSET_HASH__ replaced by the
// build-time web version so CACHE_NAME changes automatically on each deploy.
// Cache-Control: no-cache ensures the browser always revalidates the SW file.
func (h *Handler) ServeServiceWorker(w http.ResponseWriter, r *http.Request) {
	raw, _ := fs.ReadFile(web.Files, "sw.js")
	body := strings.ReplaceAll(string(raw), "__ASSET_HASH__", h.webVersion)
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprint(w, body)
}
