package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCounterAndGaugeRender(t *testing.T) {
	r := New()
	r.CounterAdd("test_total", "Test counter.", 2, []string{"kind"}, []string{"a"})
	r.CounterAdd("test_total", "Test counter.", 3, []string{"kind"}, []string{"a"})
	r.CounterAdd("test_total", "Test counter.", 1, []string{"kind"}, []string{"b"})
	r.GaugeFunc("test_depth", "Test gauge.", func() float64 { return 7 })

	out := r.Render()
	for _, want := range []string{
		"# HELP test_total Test counter.",
		"# TYPE test_total counter",
		`test_total{kind="a"} 5`,
		`test_total{kind="b"} 1`,
		"# TYPE test_depth gauge",
		"test_depth 7",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q in:\n%s", want, out)
		}
	}
}

func TestHistogramBucketsAndSum(t *testing.T) {
	r := New()
	r.RegisterBuckets("short_seconds", "Short histogram.", []float64{0.5, 1})
	r.Observe("short_seconds", "Short histogram.", 0.25, nil, nil)
	r.Observe("short_seconds", "Short histogram.", 2, nil, nil)

	out := r.Render()
	for _, want := range []string{
		`short_seconds_bucket{le="0.5"} 1`,
		`short_seconds_bucket{le="1"} 1`,
		`short_seconds_bucket{le="+Inf"} 2`,
		"short_seconds_sum 2.25",
		"short_seconds_count 2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q in:\n%s", want, out)
		}
	}
}

func TestHistogramRendersLabelsOnEverySeries(t *testing.T) {
	r := New()
	r.RegisterBuckets("labeled_seconds", "Labeled histogram.", []float64{1})
	r.Observe("labeled_seconds", "Labeled histogram.", 0.5, []string{"route"}, []string{"/videos"})

	out := r.Render()
	for _, want := range []string{
		`labeled_seconds_bucket{le="1",route="/videos"} 1`,
		`labeled_seconds_bucket{le="+Inf",route="/videos"} 1`,
		`labeled_seconds_sum{route="/videos"} 0.5`,
		`labeled_seconds_count{route="/videos"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q in:\n%s", want, out)
		}
	}
}
func TestDefaultBucketsOnObserve(t *testing.T) {
	r := New()
	r.Observe("auto_seconds", "Auto histogram.", 0.4, nil, nil)
	if !strings.Contains(r.Render(), `auto_seconds_bucket{le="0.5"} 1`) {
		t.Fatalf("default bucket missing:\n%s", r.Render())
	}
}

func TestHandlerContentTypeAndBody(t *testing.T) {
	r := New()
	r.CounterAdd("handled_total", "Handled.", 1, nil, nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("unexpected content type %q", got)
	}
	if !strings.Contains(rec.Body.String(), "handled_total 1") {
		t.Fatalf("body missing series:\n%s", rec.Body.String())
	}
}

func TestMediaHelpers(t *testing.T) {
	r := New()
	r.MediaJobClaimed()
	r.MediaJobFailed("probe")
	r.MediaJobCompleted(1500)

	out := r.Render()
	for _, want := range []string{
		`media_jobs_total{event="claimed"} 1`,
		`media_jobs_total{event="failed:probe"} 1`,
		`media_jobs_total{event="completed"} 1`,
		"media_job_duration_seconds_sum 1.5",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q in:\n%s", want, out)
		}
	}
}

func TestCounterRejectsMismatchedLabelNames(t *testing.T) {
	r := New()
	r.CounterAdd("requests_total", "Requests.", 1, []string{"route"}, []string{"/videos"})
	r.CounterAdd("requests_total", "Requests.", 2, []string{"method"}, []string{"GET"})

	out := r.Render()
	if !strings.Contains(out, `requests_total{route="/videos"} 1`) || strings.Contains(out, `method=`) {
		t.Fatalf("mismatched labels changed counter:\n%s", out)
	}
}

func TestHistogramRejectsMismatchedLabelNames(t *testing.T) {
	r := New()
	r.Observe("latency_seconds", "Latency.", 1, []string{"route"}, []string{"/videos"})
	r.Observe("latency_seconds", "Latency.", 2, []string{"method"}, []string{"GET"})

	out := r.Render()
	if !strings.Contains(out, `latency_seconds_count{route="/videos"} 1`) || strings.Contains(out, `method=`) {
		t.Fatalf("mismatched labels changed histogram:\n%s", out)
	}
}

func TestLabelsAreEscaped(t *testing.T) {
	r := New()
	r.CounterAdd("escaped_total", "Escaped.", 1, []string{"value"}, []string{"a\\b\"c\nd"})
	if out := r.Render(); !strings.Contains(out, `escaped_total{value="a\\b\"c\nd"} 1`) {
		t.Fatalf("label was not escaped:\n%s", out)
	}
}

func TestGaugePanicDoesNotAbortRender(t *testing.T) {
	r := New()
	r.GaugeFunc("broken_gauge", "Broken.", func() float64 { panic("boom") })
	r.GaugeFunc("healthy_gauge", "Healthy.", func() float64 { return 4 })
	out := r.Render()
	if !strings.Contains(out, "broken_gauge -1") || !strings.Contains(out, "healthy_gauge 4") {
		t.Fatalf("panic-safe gauge rendering failed:\n%s", out)
	}
}

func TestBlockingGaugeDoesNotHoldRegistryLock(t *testing.T) {
	r := New()
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	r.GaugeFunc("blocking_gauge", "Blocking.", func() float64 {
		once.Do(func() { close(started) })
		<-release
		return 1
	})
	rendered := make(chan struct{})
	go func() {
		r.Render()
		close(rendered)
	}()
	<-started

	added := make(chan struct{})
	go func() {
		r.CounterAdd("during_scrape_total", "Concurrent.", 1, nil, nil)
		close(added)
	}()
	select {
	case <-added:
	case <-time.After(time.Second):
		t.Fatal("CounterAdd blocked while gauge callback was running")
	}
	close(release)
	select {
	case <-rendered:
	case <-time.After(time.Second):
		t.Fatal("Render did not finish after gauge was released")
	}
}
