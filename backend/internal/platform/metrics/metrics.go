// Package metrics implements a minimal, dependency-free metrics registry
// exposing the Prometheus text exposition format. It intentionally avoids
// external clients so the backend keeps a pure-Go dependency footprint.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var defaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// Registry aggregates counters, histograms and gauges. All methods are safe
// for concurrent use.
type Registry struct {
	mu         sync.Mutex
	counters   map[string]*counterSeries
	histograms map[string]*histogramSeries
	gauges     map[string]gaugeSeries
}

type counterSeries struct {
	help       string
	labelNames []string
	values     map[string]float64
}

type histogramSeries struct {
	help       string
	labelNames []string
	buckets    []float64
	counts     map[string][]uint64
	sums       map[string]float64
	totals     map[string]uint64
}

type gaugeSeries struct {
	help string
	fn   func() float64
}

type registrySnapshot struct {
	counters   map[string]counterSeries
	histograms map[string]histogramSeries
	gauges     map[string]gaugeSeries
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{
		counters:   map[string]*counterSeries{},
		histograms: map[string]*histogramSeries{},
		gauges:     map[string]gaugeSeries{},
	}
}

// CounterAdd increments the named counter by delta for the given label values.
// help and labelNames are adopted from the first call.
func (r *Registry) CounterAdd(name, help string, delta float64, labelNames, labelValues []string) {
	if len(labelNames) != len(labelValues) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	series, ok := r.counters[name]
	if !ok {
		series = &counterSeries{help: help, labelNames: append([]string(nil), labelNames...), values: map[string]float64{}}
		r.counters[name] = series
	} else if !sameLabels(series.labelNames, labelNames) {
		return
	}
	series.values[labelKey(labelValues)] += delta
}

// Observe records value into the named histogram for the given label values.
// Default buckets are used unless RegisterBuckets was called first.
func (r *Registry) Observe(name, help string, value float64, labelNames, labelValues []string) {
	if len(labelNames) != len(labelValues) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	series, ok := r.histograms[name]
	if !ok {
		series = &histogramSeries{
			help:       help,
			labelNames: append([]string(nil), labelNames...),
			buckets:    append([]float64(nil), defaultBuckets...),
			counts:     map[string][]uint64{},
			sums:       map[string]float64{},
			totals:     map[string]uint64{},
		}
		r.histograms[name] = series
	} else if len(series.labelNames) == 0 && len(series.totals) == 0 {
		series.labelNames = append([]string(nil), labelNames...)
	} else if !sameLabels(series.labelNames, labelNames) {
		return
	}
	key := labelKey(labelValues)
	if _, ok := series.counts[key]; !ok {
		series.counts[key] = make([]uint64, len(series.buckets))
	}
	for i, bound := range series.buckets {
		if value <= bound {
			series.counts[key][i]++
		}
	}
	series.sums[key] += value
	series.totals[key]++
}

// RegisterBuckets sets custom buckets for a histogram. Call before first Observe.
func (r *Registry) RegisterBuckets(name, help string, buckets []float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.histograms[name]; ok {
		return
	}
	r.histograms[name] = &histogramSeries{
		help:    help,
		buckets: append([]float64(nil), buckets...),
		counts:  map[string][]uint64{},
		sums:    map[string]float64{},
		totals:  map[string]uint64{},
	}
}

// GaugeFunc registers a gauge whose value is read at scrape time.
func (r *Registry) GaugeFunc(name, help string, fn func() float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauges[name] = gaugeSeries{help: help, fn: fn}
}

// Handler serves the registry in Prometheus text exposition format.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Write([]byte(r.Render()))
	})
}

// Render returns the current registry snapshot in text exposition format.
func (r *Registry) Render() string {
	snapshot := r.snapshot()
	var b strings.Builder
	writeFamily := func(name, help, typeName string, render func(*strings.Builder)) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, typeName)
		render(&b)
	}

	for _, name := range sortedKeys(snapshot.counters) {
		series := snapshot.counters[name]
		writeFamily(name, series.help, "counter", func(b *strings.Builder) {
			for _, key := range sortedKeys(series.values) {
				b.WriteString(name)
				writeLabels(b, series.labelNames, key)
				fmt.Fprintf(b, " %s\n", formatFloat(series.values[key]))
			}
		})
	}
	for _, name := range sortedKeys(snapshot.histograms) {
		series := snapshot.histograms[name]
		writeFamily(name, series.help, "histogram", func(b *strings.Builder) {
			for _, key := range sortedKeys(series.counts) {
				for i, bound := range series.buckets {
					fmt.Fprintf(b, "%s_bucket{le=\"%s\"", name, formatFloat(bound))
					writeAdditionalLabels(b, series.labelNames, key)
					fmt.Fprintf(b, "} %d\n", series.counts[key][i])
				}
				fmt.Fprintf(b, "%s_bucket{le=\"+Inf\"", name)
				writeAdditionalLabels(b, series.labelNames, key)
				fmt.Fprintf(b, "} %d\n", series.totals[key])
				b.WriteString(name + "_sum")
				writeLabels(b, series.labelNames, key)
				fmt.Fprintf(b, " %s\n", formatFloat(series.sums[key]))
				b.WriteString(name + "_count")
				writeLabels(b, series.labelNames, key)
				fmt.Fprintf(b, " %d\n", series.totals[key])
			}
		})
	}
	for _, name := range sortedKeys(snapshot.gauges) {
		gauge := snapshot.gauges[name]
		writeFamily(name, gauge.help, "gauge", func(b *strings.Builder) {
			fmt.Fprintf(b, "%s %s\n", name, formatFloat(readGauge(gauge.fn)))
		})
	}
	return b.String()
}

func (r *Registry) snapshot() registrySnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot := registrySnapshot{
		counters:   make(map[string]counterSeries, len(r.counters)),
		histograms: make(map[string]histogramSeries, len(r.histograms)),
		gauges:     make(map[string]gaugeSeries, len(r.gauges)),
	}
	for name, series := range r.counters {
		values := make(map[string]float64, len(series.values))
		for key, value := range series.values {
			values[key] = value
		}
		snapshot.counters[name] = counterSeries{help: series.help, labelNames: append([]string(nil), series.labelNames...), values: values}
	}
	for name, series := range r.histograms {
		counts := make(map[string][]uint64, len(series.counts))
		for key, value := range series.counts {
			counts[key] = append([]uint64(nil), value...)
		}
		sums := make(map[string]float64, len(series.sums))
		for key, value := range series.sums {
			sums[key] = value
		}
		totals := make(map[string]uint64, len(series.totals))
		for key, value := range series.totals {
			totals[key] = value
		}
		snapshot.histograms[name] = histogramSeries{help: series.help, labelNames: append([]string(nil), series.labelNames...), buckets: append([]float64(nil), series.buckets...), counts: counts, sums: sums, totals: totals}
	}
	for name, gauge := range r.gauges {
		snapshot.gauges[name] = gauge
	}
	return snapshot
}

func readGauge(fn func() float64) (value float64) {
	value = -1
	defer func() { _ = recover() }()
	return fn()
}

// HTTP metric helpers -------------------------------------------------------

// IncHTTP counts one request with method, route pattern and status labels.
func (r *Registry) IncHTTP(method, route string, status int) {
	r.CounterAdd("http_requests_total", "Total HTTP requests processed.", 1,
		[]string{"method", "route", "status"},
		[]string{method, route, strconv.Itoa(status)})
}

// ObserveHTTPDuration records request latency in seconds.
func (r *Registry) ObserveHTTPDuration(route string, seconds float64) {
	r.Observe("http_request_duration_seconds", "HTTP request latency in seconds.", seconds,
		[]string{"route"}, []string{route})
}

// MediaJobClaimed counts a claimed transcoding job.
func (r *Registry) MediaJobClaimed() {
	r.CounterAdd("media_jobs_total", "Transcoding job lifecycle events.", 1,
		[]string{"event"}, []string{"claimed"})
}

// MediaJobCompleted records a finished transcoding job and its duration.
func (r *Registry) MediaJobCompleted(duration time.Duration) {
	r.CounterAdd("media_jobs_total", "Transcoding job lifecycle events.", 1,
		[]string{"event"}, []string{"completed"})
	r.Observe("media_job_duration_seconds", "Transcoding job duration in seconds.", duration.Seconds(), nil, nil)
}

// MediaJobFailed counts a failed transcoding job at the given pipeline stage.
func (r *Registry) MediaJobFailed(stage string) {
	r.CounterAdd("media_jobs_total", "Transcoding job lifecycle events.", 1,
		[]string{"event"}, []string{"failed:" + stage})
}

// internals -----------------------------------------------------------------

func labelKey(values []string) string { return strings.Join(values, "\x1f") }

func sameLabels(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func writeLabels(b *strings.Builder, names []string, key string) {
	if len(names) == 0 {
		return
	}
	b.WriteByte('{')
	values := strings.Split(key, "\x1f")
	for i, name := range names {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(b, "%s=\"%s\"", name, escapeLabel(values[i]))
	}
	b.WriteByte('}')
}

func writeAdditionalLabels(b *strings.Builder, names []string, key string) {
	if len(names) == 0 {
		return
	}
	values := strings.Split(key, "\x1f")
	for i, name := range names {
		b.WriteByte(',')
		fmt.Fprintf(b, "%s=\"%s\"", name, escapeLabel(values[i]))
	}
}

func escapeLabel(v string) string {
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n")
	return r.Replace(v)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
