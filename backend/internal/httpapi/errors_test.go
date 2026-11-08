package httpapi

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSONFailsVisiblyOnUnencodableValue(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSON(recorder, http.StatusOK, map[string]float64{"value": math.Inf(1)})
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for an unencodable value, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "INTERNAL_ERROR") {
		t.Fatalf("expected a stable error code, got %q", recorder.Body.String())
	}
}
