// Simple app with health and readiness checks
// and Prometheus metrics for Kubernetes testing
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func Test_sanitizeVendor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string defaults to unknown", "", "unknown"},
		{"valid lowercase", "acme", "acme"},
		{"valid with hyphen", "my-vendor", "my-vendor"},
		{"valid with underscore", "my_vendor", "my_vendor"},
		{"mixed case preserved", "AcmeCorp", "AcmeCorp"},
		{"strips special characters", "acme!@#$%", "acme"},
		{"all invalid chars defaults to unknown", "!@#$%^&*()", "unknown"},
		{"truncates at 64 chars", strings.Repeat("a", 100), strings.Repeat("a", 64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeVendor(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeVendor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func Test_checkRest(t *testing.T) {
	// checkRest randomly returns 200 OK or 502 Bad Gateway — both are valid outcomes.
	validCodes := map[int]bool{
		http.StatusOK:         true,
		http.StatusBadGateway: true,
	}
	tests := []struct {
		name   string
		url    string
	}{
		{"with named vendor", "/checkrest?vendor=acme"},
		{"no vendor defaults to unknown", "/checkrest"},
		{"vendor with special chars is sanitized", "/checkrest?vendor=acme!injected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			checkRest(w, r)
			if !validCodes[w.Code] {
				t.Errorf("checkRest status = %d, want one of %v", w.Code, []int{http.StatusOK, http.StatusBadGateway})
			}
		})
	}
}

func TestRecordCurlError(t *testing.T) {
	tests := []struct {
		name   string
		vendor string
	}{
		{"known vendor", "acme"},
		{"unknown vendor", "unknown"},
		{"numeric vendor", "vendor123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordCurlError(tt.vendor)
		})
	}
}

func Test_echoString(t *testing.T) {
	// Note: echoString sleeps up to 1000ms per call to simulate load.

	t.Run("root path returns 200 with hostname and counter", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		echoString(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		body := w.Body.String()
		if !strings.Contains(body, "I am:") {
			t.Errorf("body missing hostname line, got: %q", body)
		}
		if !strings.Contains(body, "Requests:") {
			t.Errorf("body missing requests line, got: %q", body)
		}
		if !strings.Contains(body, "Time:") {
			t.Errorf("body missing time line, got: %q", body)
		}
	})

	t.Run("non-root path returns 404", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/other", nil)
		w := httptest.NewRecorder()
		echoString(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("counter increments across requests", func(t *testing.T) {
		before := counter
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		echoString(w, r)
		if counter != before+1 {
			t.Errorf("counter = %d, want %d", counter, before+1)
		}
	})
}

func Test_healthz(t *testing.T) {
	t.Run("returns 200 with OK body", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		w := httptest.NewRecorder()
		healthz(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Body.String(), "OK") {
			t.Errorf("body = %q, want to contain \"OK\"", w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Errorf("Content-Type = %q, want \"text/plain; charset=utf-8\"", ct)
		}
	})
}

func Test_readyz(t *testing.T) {
	tests := []struct {
		name     string
		ready    *bool
		wantCode int
	}{
		{"nil isReady returns 503", nil, http.StatusServiceUnavailable},
		{"isReady=false returns 503", boolPtr(false), http.StatusServiceUnavailable},
		{"isReady=true returns 200", boolPtr(true), http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			w := httptest.NewRecorder()

			var isReady *atomic.Value
			if tt.ready != nil {
				isReady = &atomic.Value{}
				isReady.Store(*tt.ready)
			}

			readyz(w, r, isReady)
			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}

func Test_faviconHandler(t *testing.T) {
	t.Run("returns image/x-icon with non-empty body", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
		w := httptest.NewRecorder()
		faviconHandler(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/x-icon" {
			t.Errorf("Content-Type = %q, want \"image/x-icon\"", ct)
		}
		if w.Body.Len() == 0 {
			t.Error("body is empty, expected favicon bytes")
		}
	})
}

// boolPtr returns a pointer to a bool literal.
func boolPtr(b bool) *bool { return &b }
