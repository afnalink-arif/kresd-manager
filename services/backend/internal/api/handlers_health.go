package api

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	status := "healthy"
	checks := map[string]string{}

	// Check ClickHouse
	if err := s.ch.Ping(ctx); err != nil {
		checks["clickhouse"] = "error: " + err.Error()
		status = "degraded"
	} else {
		checks["clickhouse"] = "ok"
	}

	// Check PostgreSQL
	if err := s.pg.Ping(ctx); err != nil {
		checks["postgres"] = "error: " + err.Error()
		status = "degraded"
	} else {
		checks["postgres"] = "ok"
	}

	// Check Redis
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		checks["redis"] = "error: " + err.Error()
		status = "degraded"
	} else {
		checks["redis"] = "ok"
	}

	// Check Prometheus
	resp, err := s.httpClient.Get(s.promURL + "/-/healthy")
	if err != nil {
		checks["prometheus"] = "error: " + err.Error()
		status = "degraded"
	} else {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			checks["prometheus"] = "ok"
		} else {
			checks["prometheus"] = "error: status " + resp.Status
			status = "degraded"
		}
	}

	code := http.StatusOK
	if status == "degraded" {
		code = http.StatusServiceUnavailable
	}

	w.WriteHeader(code)
	writeJSON(w, map[string]interface{}{
		"status": status,
		"checks": checks,
	})
}
