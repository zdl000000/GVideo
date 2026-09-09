package httpapi

import (
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// errorContractEntry is the executable registry for docs/api-error-contract.md.
// Production adoption is deliberately deferred to a separate implementation task.
type errorContractEntry struct {
	Code   string
	Status int
}

var errorContractRegistry = []errorContractEntry{
	{Code: "invalid_request", Status: http.StatusBadRequest},
	{Code: "invalid_input", Status: http.StatusBadRequest},
	{Code: "authentication_required", Status: http.StatusUnauthorized},
	{Code: "invalid_credentials", Status: http.StatusUnauthorized},
	{Code: "forbidden", Status: http.StatusForbidden},
	{Code: "csrf_failed", Status: http.StatusForbidden},
	{Code: "not_found", Status: http.StatusNotFound},
	{Code: "conflict", Status: http.StatusConflict},
	{Code: "subtitle_exists", Status: http.StatusConflict},
	{Code: "video_processing", Status: http.StatusConflict},
	{Code: "retry_unavailable", Status: http.StatusConflict},
	{Code: "report_exists", Status: http.StatusConflict},
	{Code: "internal_error", Status: http.StatusInternalServerError},
}

type contractProblem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type contractErrorEnvelope struct {
	Error     contractProblem `json:"error"`
	RequestID string          `json:"request_id"`
}

type contractSuccessEnvelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

func TestErrorContractRegistry(t *testing.T) {
	codePattern := regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)
	got := make(map[string]int, len(errorContractRegistry))

	for _, entry := range errorContractRegistry {
		if !codePattern.MatchString(entry.Code) {
			t.Fatalf("code %q is not lower snake_case", entry.Code)
		}
		if _, exists := got[entry.Code]; exists {
			t.Fatalf("duplicate error code %q", entry.Code)
		}
		got[entry.Code] = entry.Status
	}

	want := map[string]int{
		"authentication_required": http.StatusUnauthorized,
		"conflict":                http.StatusConflict,
		"csrf_failed":             http.StatusForbidden,
		"forbidden":               http.StatusForbidden,
		"internal_error":          http.StatusInternalServerError,
		"invalid_credentials":     http.StatusUnauthorized,
		"invalid_input":           http.StatusBadRequest,
		"invalid_request":         http.StatusBadRequest,
		"not_found":               http.StatusNotFound,
		"report_exists":           http.StatusConflict,
		"retry_unavailable":       http.StatusConflict,
		"subtitle_exists":         http.StatusConflict,
		"video_processing":        http.StatusConflict,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("error contract registry mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestErrorContractEnvelopeShape(t *testing.T) {
	encoded, err := json.Marshal(contractErrorEnvelope{
		Error: contractProblem{
			Code:    "invalid_request",
			Message: "请求内容不是有效的 JSON",
		},
		RequestID: "req-contract-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"error": map[string]any{
			"code":    "invalid_request",
			"message": "请求内容不是有效的 JSON",
		},
		"request_id": "req-contract-1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("error envelope mismatch\n got: %#v\nwant: %#v", got, want)
	}
	if _, exists := got["data"]; exists {
		t.Fatal("error envelope must not contain data")
	}
}

func TestSuccessContractEnvelopeShape(t *testing.T) {
	encoded, err := json.Marshal(contractSuccessEnvelope{
		Data:      map[string]any{"status": "ok"},
		RequestID: "req-contract-2",
	})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"data":       map[string]any{"status": "ok"},
		"request_id": "req-contract-2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("success envelope mismatch\n got: %#v\nwant: %#v", got, want)
	}
	if _, exists := got["error"]; exists {
		t.Fatal("success envelope must not contain error")
	}
}

func TestRequestIDContract(t *testing.T) {
	valid := []string{
		"a",
		"4e1d7d8d913e45869055b94e",
		"trace.v1:edge-01_request",
		strings.Repeat("a", 128),
	}
	for _, value := range valid {
		if !isContractRequestID(value) {
			t.Errorf("request ID %q should be valid", value)
		}
	}

	invalid := []string{
		"",
		" leading-space",
		"trailing-space ",
		"line\nbreak",
		"包含中文",
		"path/segment",
		strings.Repeat("a", 129),
	}
	for _, value := range invalid {
		if isContractRequestID(value) {
			t.Errorf("request ID %q should be rejected", value)
		}
	}

	const requestID = "req-contract-match"
	header := make(http.Header)
	header.Set("X-Request-ID", requestID)
	envelope := contractErrorEnvelope{
		Error:     contractProblem{Code: "internal_error", Message: "服务暂时不可用"},
		RequestID: requestID,
	}
	if header.Get("X-Request-ID") != envelope.RequestID {
		t.Fatalf("response header request ID %q differs from envelope %q", header.Get("X-Request-ID"), envelope.RequestID)
	}
}

func isContractRequestID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	return regexp.MustCompile(`^[A-Za-z0-9._:-]+$`).MatchString(value)
}
