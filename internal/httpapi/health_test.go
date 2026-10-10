package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	h := Handler()
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/health/live", 200}, {"GET", "/health/ready", 503}, {"POST", "/health/live", 405}, {"POST", "/health/ready", 405}, {"GET", "/api/v1/sms", 404}, {"GET", "/api/v1/otp", 404}, {"GET", "/health/live/extra", 404}} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.status {
				t.Fatalf("status %d", w.Code)
			}
			if tc.status == 200 || tc.status == 503 {
				if w.Header().Get("Content-Type") != "application/json" {
					t.Fatal("missing JSON content type")
				}
			}
			if tc.status == 503 {
				var body struct {
					Error struct {
						Code, Message string
						RequestID     string `json:"request_id"`
					}
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Error.Code != "storage_not_initialized" || body.Error.RequestID == "" || body.Error.RequestID != w.Header().Get("X-Request-ID") {
					t.Fatalf("unexpected readiness: %s", w.Body.String())
				}
			}
		})
	}
}
