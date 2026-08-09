package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qutaq/GophProfile/internal/handlers"
)

func TestHealthDegradedWithoutDeps(t *testing.T) {
	h := handlers.NewHealthHandler(handlers.HealthDeps{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.Health(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "degraded" {
		t.Fatalf("body status = %#v", body["status"])
	}
	components, ok := body["components"].(map[string]any)
	if !ok || components["postgres"] == nil || components["minio"] == nil || components["rabbitmq"] == nil {
		t.Fatalf("components = %#v", body["components"])
	}
}
