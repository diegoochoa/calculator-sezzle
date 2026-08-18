package httpx

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusCreated, map[string]int{"result": 42})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}

	var body map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["result"] != 42 {
		t.Errorf("body = %v", body)
	}
}

// A NaN cannot be represented in JSON. The encoder must fail before the status
// line is written so the client sees a 500 rather than a truncated 200.
func TestWriteJSONUnencodableFailsCleanly(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, map[string]float64{"result": math.NaN()})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestWriteErrorCarriesRequestID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/v1/calculate", nil)
	req = req.WithContext(WithRequestID(req.Context(), "req-123"))

	rec := httptest.NewRecorder()
	WriteError(rec, req, http.StatusBadRequest, CodeInvalidRequest, "bad input")

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body.Error.Code != CodeInvalidRequest || body.Error.Message != "bad input" {
		t.Errorf("error = %+v", body.Error)
	}
	if body.RequestID != "req-123" {
		t.Errorf("requestId = %q, want req-123", body.RequestID)
	}
	if body.Error.Position != nil {
		t.Errorf("position = %v, want omitted", *body.Error.Position)
	}
}

func TestWriteErrorAtIncludesPosition(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/v1/calculate", nil)
	rec := httptest.NewRecorder()
	WriteErrorAt(rec, req, http.StatusUnprocessableEntity, "DIVISION_BY_ZERO", "Can't divide by zero", 4)

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body.Error.Position == nil || *body.Error.Position != 4 {
		t.Errorf("position = %v, want 4", body.Error.Position)
	}
	// No middleware ran, so the id is absent rather than empty-stringed in.
	if body.RequestID != "" {
		t.Errorf("requestId = %q, want empty", body.RequestID)
	}
}

func TestContextValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	if RequestID(ctx) != "" || Subject(ctx) != "" {
		t.Fatal("empty context should yield empty values")
	}

	ctx = WithRequestID(ctx, "abc")
	ctx = WithSubject(ctx, "web")
	if got := RequestID(ctx); got != "abc" {
		t.Errorf("RequestID = %q, want abc", got)
	}
	if got := Subject(ctx); got != "web" {
		t.Errorf("Subject = %q, want web", got)
	}
}
