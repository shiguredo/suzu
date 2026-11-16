package suzu

import (
	"encoding/json"
	"net/http"
)

// healthcheckHandler は LB からのヘルスチェックにバージョンを JSON で返す
// ステータスコードは最初の書き込みで 200 に確定する
func (s *Server) healthcheckHandler(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"version": s.config.Version,
	})
}
