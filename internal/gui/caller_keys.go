package gui

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/budget"
)

// keyRow is a gateway key as the Gateway page lists it, with what it has
// used of its limit (#585).
type keyRow struct {
	access.Key
	Used *budget.Status `json:"used,omitempty"`
}

func withLimits(keys []access.Key) []keyRow {
	now := time.Now()
	out := make([]keyRow, 0, len(keys))
	for _, k := range keys {
		out = append(out, keyRow{Key: k, Used: budget.Of(k, now)})
	}
	return out
}

func callerKeyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/caller-keys", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		access.MigrateLegacyLANKeyBestEffort()
		keys, err := access.List()
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, map[string]any{"keys": withLimits(keys)})
	})
	mux.HandleFunc("POST /api/caller-keys/{action}", func(w http.ResponseWriter, r *http.Request) {
		var in access.Change
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			fail(w, err)
			return
		}
		access.MigrateLegacyLANKeyBestEffort()
		secret, err := access.Update(r.PathValue("action"), in)
		if err != nil {
			fail(w, err)
			return
		}
		keys, err := access.List()
		if err != nil {
			fail(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, map[string]any{"keys": withLimits(keys), "secret": secret})
	})
}
