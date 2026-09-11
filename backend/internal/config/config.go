package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                    string
	HTTPAddr                  string
	FrontendURL               string
	AdminUsername             string
	DatabasePath              string
	MediaDir                  string
	SessionTTL                time.Duration
	MaxUploadBytes            int64
	CookieSecure              bool
	FFmpegPath                string
	FFprobePath               string
	MediaWorkerEnabled        bool
	MediaWorkerPollInterval   time.Duration
	MediaProbeTimeout         time.Duration
	HLSEnabled                bool
	HLSTranscodeTimeout       time.Duration
	HLSSegmentSeconds         int
	MetricsAddr               string
	PprofAddr                 string
	RateLimitAuthPerMinute    int
	RateLimitCommentPerMinute int
	RateLimitUploadPerMinute  int
	UserStorageQuotaBytes     int64
}

func Load() (Config, error) {
	appEnv := strings.ToLower(env("APP_ENV", "development"))
	frontendURL := env("FRONTEND_URL", "http://127.0.0.1:5173")
	parsedFrontendURL, err := url.Parse(frontendURL)
	if err != nil || (parsedFrontendURL.Scheme != "http" && parsedFrontendURL.Scheme != "https") || parsedFrontendURL.Host == "" || parsedFrontendURL.User != nil || (parsedFrontendURL.Path != "" && parsedFrontendURL.Path != "/") || parsedFrontendURL.RawQuery != "" || parsedFrontendURL.Fragment != "" {
		return Config{}, fmt.Errorf("FRONTEND_URL must be an HTTP(S) origin without credentials, path, query, or fragment")
	}
	ttl, err := time.ParseDuration(env("SESSION_TTL", "168h"))
	if err != nil {
		return Config{}, fmt.Errorf("SESSION_TTL: %w", err)
	}
	maxUpload, err := strconv.ParseInt(env("MAX_UPLOAD_BYTES", "536870912"), 10, 64)
	if err != nil || maxUpload <= 0 {
		return Config{}, fmt.Errorf("MAX_UPLOAD_BYTES must be a positive integer")
	}
	secure, err := strconv.ParseBool(env("COOKIE_SECURE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("COOKIE_SECURE: %w", err)
	}
	if appEnv == "production" && (!secure || parsedFrontendURL.Scheme != "https") {
		return Config{}, fmt.Errorf("production requires COOKIE_SECURE=true and an HTTPS FRONTEND_URL")
	}
	workerEnabled, err := strconv.ParseBool(env("MEDIA_WORKER_ENABLED", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("MEDIA_WORKER_ENABLED: %w", err)
	}
	pollInterval, err := time.ParseDuration(env("MEDIA_WORKER_POLL_INTERVAL", "2s"))
	if err != nil || pollInterval <= 0 {
		return Config{}, fmt.Errorf("MEDIA_WORKER_POLL_INTERVAL must be a positive duration")
	}
	probeTimeout, err := time.ParseDuration(env("MEDIA_PROBE_TIMEOUT", "30s"))
	if err != nil || probeTimeout <= 0 {
		return Config{}, fmt.Errorf("MEDIA_PROBE_TIMEOUT must be a positive duration")
	}
	hlsEnabled, err := strconv.ParseBool(env("HLS_ENABLED", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("HLS_ENABLED: %w", err)
	}
	hlsTimeout, err := time.ParseDuration(env("HLS_TRANSCODE_TIMEOUT", "30m"))
	if err != nil || hlsTimeout <= 0 {
		return Config{}, fmt.Errorf("HLS_TRANSCODE_TIMEOUT must be a positive duration")
	}
	hlsSegmentSeconds, err := strconv.Atoi(env("HLS_SEGMENT_SECONDS", "4"))
	if err != nil || hlsSegmentSeconds < 2 || hlsSegmentSeconds > 10 {
		return Config{}, fmt.Errorf("HLS_SEGMENT_SECONDS must be between 2 and 10")
	}
	metricsAddr := strings.TrimSpace(os.Getenv("METRICS_ADDR"))
	pprofAddr := strings.TrimSpace(os.Getenv("PPROF_ADDR"))
	rateLimitAuth, err := intEnv("RATE_LIMIT_AUTH_PER_MINUTE", 20)
	if err != nil {
		return Config{}, err
	}
	rateLimitComment, err := intEnv("RATE_LIMIT_COMMENT_PER_MINUTE", 30)
	if err != nil {
		return Config{}, err
	}
	rateLimitUpload, err := intEnv("RATE_LIMIT_UPLOAD_PER_MINUTE", 10)
	if err != nil {
		return Config{}, err
	}
	storageQuotaBytes, err := strconv.ParseInt(env("USER_STORAGE_QUOTA_BYTES", "0"), 10, 64)
	if err != nil || storageQuotaBytes < 0 {
		return Config{}, fmt.Errorf("USER_STORAGE_QUOTA_BYTES must be a non-negative integer")
	}
	if appEnv == "production" {
		if err := validateProductionDiagnosticsAddress("METRICS_ADDR", metricsAddr); err != nil {
			return Config{}, err
		}
		if err := validateProductionDiagnosticsAddress("PPROF_ADDR", pprofAddr); err != nil {
			return Config{}, err
		}
	}

	databasePath, err := filepath.Abs(env("DATABASE_PATH", "./data/gvideo.db"))
	if err != nil {
		return Config{}, fmt.Errorf("resolve database path: %w", err)
	}
	mediaDir, err := filepath.Abs(env("MEDIA_DIR", "./media"))
	if err != nil {
		return Config{}, fmt.Errorf("resolve media dir: %w", err)
	}

	return Config{
		AppEnv:                    appEnv,
		HTTPAddr:                  env("HTTP_ADDR", ":8080"),
		FrontendURL:               strings.TrimSuffix(frontendURL, "/"),
		AdminUsername:             env("ADMIN_USERNAME", ""),
		DatabasePath:              databasePath,
		MediaDir:                  mediaDir,
		SessionTTL:                ttl,
		MaxUploadBytes:            maxUpload,
		CookieSecure:              secure,
		FFmpegPath:                env("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:               env("FFPROBE_PATH", "ffprobe"),
		MediaWorkerEnabled:        workerEnabled,
		MediaWorkerPollInterval:   pollInterval,
		MediaProbeTimeout:         probeTimeout,
		HLSEnabled:                hlsEnabled,
		HLSTranscodeTimeout:       hlsTimeout,
		HLSSegmentSeconds:         hlsSegmentSeconds,
		MetricsAddr:               metricsAddr,
		PprofAddr:                 pprofAddr,
		RateLimitAuthPerMinute:    rateLimitAuth,
		RateLimitCommentPerMinute: rateLimitComment,
		RateLimitUploadPerMinute:  rateLimitUpload,
		UserStorageQuotaBytes:     storageQuotaBytes,
	}, nil
}

// intEnv reads a non-negative integer setting; zero disables the feature the
// setting belongs to.
func intEnv(key string, fallback int) (int, error) {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return value, nil
}

func validateProductionDiagnosticsAddress(name, address string) error {
	if address == "" {
		return nil
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s must use host:port format: %w", name, err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("%s port must be between 1 and 65535", name)
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("%s must not bind an unspecified or wildcard address in production", name)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}

	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%s must use localhost or a literal loopback/private IP address in production", name)
	}
	ip = ip.Unmap()
	if ip.IsUnspecified() || (!ip.IsLoopback() && !ip.IsPrivate()) {
		return fmt.Errorf("%s must use a loopback or private IP address in production", name)
	}
	return nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
