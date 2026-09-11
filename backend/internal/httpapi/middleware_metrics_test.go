package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"gvideo/backend/internal/platform/metrics"
)

func TestInstrumentRecordsRouteAndCommittedStatus(t *testing.T) {
	registry := metrics.New()
	handler := &Handler{metrics: registry}
	router := chi.NewRouter()
	router.Use(handler.instrument)
	router.Get("/videos/{videoID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/videos/42", nil))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("unexpected response status %d", recorder.Code)
	}
	output := registry.Render()
	if !strings.Contains(output, `http_requests_total{method="GET",route="/videos/{videoID}",status="201"} 1`) {
		t.Fatalf("request metric missing committed status and route:\n%s", output)
	}
	if strings.Contains(output, `status="500"`) {
		t.Fatalf("request metric recorded an ignored second WriteHeader:\n%s", output)
	}
}

func TestInstrumentNormalizesImplicitOKStatus(t *testing.T) {
	registry := metrics.New()
	handler := &Handler{metrics: registry}
	router := chi.NewRouter()
	router.Use(handler.instrument)
	router.Get("/empty", func(http.ResponseWriter, *http.Request) {})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/empty", nil))

	output := registry.Render()
	if !strings.Contains(output, `http_requests_total{method="GET",route="/empty",status="200"} 1`) {
		t.Fatalf("implicit status was not normalized to 200:\n%s", output)
	}
	if strings.Contains(output, `status="0"`) {
		t.Fatalf("metrics exposed status zero:\n%s", output)
	}
}
