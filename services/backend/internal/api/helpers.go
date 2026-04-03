package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func queryOrDefault(r *http.Request, key, def string) string {
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	return def
}

func timeRangeParams(r *http.Request) url.Values {
	params := url.Values{}

	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	if start == "" {
		start = time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	}
	if end == "" {
		end = time.Now().Format(time.RFC3339)
	}

	params.Set("start", start)
	params.Set("end", end)
	return params
}
