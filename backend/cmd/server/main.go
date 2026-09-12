package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	httppprof "net/http/pprof"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/httpapi"
	"gvideo/backend/internal/media"
	"gvideo/backend/internal/modules/comments"
	"gvideo/backend/internal/modules/interactions"
	"gvideo/backend/internal/modules/moderation"
	"gvideo/backend/internal/modules/notifications"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/platform/bus"
	platformhealth "gvideo/backend/internal/platform/health"
	"gvideo/backend/internal/platform/metrics"
	"gvideo/backend/internal/repository"
	"gvideo/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 {
		if err := runDataCommand(context.Background(), cfg, os.Args[1:]); err != nil {
			logger.Error("run data command", "error", err)
			os.Exit(1)
		}
		return
	}

	db, err := platform.OpenDatabase(cfg.DatabasePath)
	if err != nil {
		logger.Error("storage initialization failed", "event", "storage_initialization_failed", "error_class", "storage_failed")
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("storage initialized", "event", "storage_initialized")

	repo := repository.New(db)
	if cfg.AdminUsername != "" {
		if !service.ValidUsername(cfg.AdminUsername) {
			logger.Error("ADMIN_USERNAME is not a valid username", "event", "admin_username_invalid")
			os.Exit(1)
		}
		hasAdmin, err := repo.HasAdmin(context.Background())
		if err != nil {
			logger.Error("inspect administrator state", "event", "admin_state_check_failed", "error", err)
			os.Exit(1)
		}
		if !hasAdmin {
			logger.Warn("ADMIN_USERNAME is configured but no administrator exists yet; register the reserved username or run data-grant-admin before public exposure", "event", "admin_unclaimed")
		}
	}
	svc := service.New(repo, cfg, logger)
	moderationService := moderation.NewService(moderation.NewRepository(db))
	// 进程内同步事件总线：通知写路径的唯一入口。订阅者与发布方同请求
	// goroutine 执行，保持「通知随请求落库」的既有语义；写失败仅记录告警。
	eventBus := bus.New()
	notificationsRepo := notifications.NewRepository(db)
	notificationsService := notifications.NewService(notificationsRepo)
	eventBus.Subscribe(func(ctx context.Context, event bus.NotificationEvent) {
		if err := notificationsRepo.Create(ctx, event.RecipientID, event.ActorID, event.Kind, event.VideoID, event.CommentID, event.VideoTitle, event.CommentPreview); err != nil {
			logger.Warn("record notification", "event", "notification_record_failed", "kind", event.Kind, "recipient_id", event.RecipientID, "error", err)
		}
	})
	commentsService := comments.NewService(comments.NewRepository(db), eventBus, logger)
	interactionsService := interactions.NewService(interactions.NewRepository(db), eventBus, logger)
	registry := metrics.New()
	handler := httpapi.New(svc, moderationService, notificationsService, commentsService, interactionsService, cfg, logger, registry).WithReadiness(platformhealth.NewReadiness(db))
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      15 * time.Minute,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

	if cfg.MetricsAddr != "" {
		if cfg.MediaWorkerEnabled {
			registry.GaugeFunc("media_queue_depth", "Transcoding jobs waiting or running.", func() float64 {
				queryCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				count, err := repo.PendingJobCount(queryCtx)
				if err != nil {
					return -1
				}
				return float64(count)
			})
		}
	}
	adminServers, err := startAdminServers(logger, cfg.MetricsAddr, cfg.PprofAddr, registry)
	if err != nil {
		logger.Error("start admin endpoints", "error", err)
		os.Exit(1)
	}
	for _, warning := range diagnosticsBindingWarnings(cfg.AppEnv, cfg.MetricsAddr, cfg.PprofAddr) {
		logger.Warn(warning, "event", "diagnostics_binding_exposed")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.MediaWorkerEnabled {
		transcoder := media.NewFFmpegTranscoder(cfg.HLSEnabled, cfg.FFmpegPath, cfg.MediaDir, cfg.HLSTranscodeTimeout, cfg.HLSSegmentSeconds)
		worker := media.NewWorker(repo, media.NewFFprobe(cfg.FFprobePath, cfg.MediaProbeTimeout), transcoder, cfg.MediaDir, cfg.MediaWorkerPollInterval, logger).WithStats(registry)
		go worker.Run(ctx)
	}

	go func() {
		logger.Info("server started", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("serve http", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown", "error", err)
	}
	for _, adminServer := range adminServers {
		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("admin server shutdown", "addr", adminServer.Addr, "error", err)
		}
	}
}

type adminEndpoint struct {
	name     string
	path     string
	server   *http.Server
	listener net.Listener
}

// diagnosticsBindingWarnings reports diagnostics endpoints that are reachable
// beyond the loopback interface outside production. Production rejects such
// bindings during configuration loading; lower environments keep the operator
// in control but should not expose pprof/metrics silently.
func diagnosticsBindingWarnings(appEnv, metricsAddr, pprofAddr string) []string {
	if appEnv == "production" {
		return nil
	}
	var warnings []string
	for _, endpoint := range []struct{ name, addr string }{
		{name: "METRICS_ADDR", addr: metricsAddr},
		{name: "PPROF_ADDR", addr: pprofAddr},
	} {
		if endpoint.addr == "" {
			continue
		}
		host, _, err := net.SplitHostPort(endpoint.addr)
		if err != nil {
			continue
		}
		host = strings.TrimSpace(host)
		if host == "" {
			warnings = append(warnings, endpoint.name+" binds every interface; prefer 127.0.0.1 outside production")
			continue
		}
		if strings.EqualFold(host, "localhost") {
			continue
		}
		if ip, err := netip.ParseAddr(host); err != nil || !ip.Unmap().IsLoopback() {
			warnings = append(warnings, endpoint.name+" binds a non-loopback address; prefer 127.0.0.1 outside production")
		}
	}
	return warnings
}

func startAdminServers(logger *slog.Logger, metricsAddr, pprofAddr string, registry *metrics.Registry) ([]*http.Server, error) {
	var endpoints []adminEndpoint
	if metricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", registry.Handler())
		endpoints = append(endpoints, adminEndpoint{
			name: "metrics",
			path: "/metrics",
			server: &http.Server{
				Addr:              metricsAddr,
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
				ReadTimeout:       5 * time.Second,
				WriteTimeout:      5 * time.Second,
				IdleTimeout:       30 * time.Second,
			},
		})
	}
	if pprofAddr != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", httppprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", httppprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", httppprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", httppprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", httppprof.Trace)
		endpoints = append(endpoints, adminEndpoint{
			name: "pprof",
			path: "/debug/pprof/",
			server: &http.Server{
				Addr:              pprofAddr,
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
				ReadTimeout:       5 * time.Second,
				WriteTimeout:      2 * time.Minute,
				IdleTimeout:       30 * time.Second,
			},
		})
	}

	for index := range endpoints {
		endpoint := &endpoints[index]
		listener, err := net.Listen("tcp", endpoint.server.Addr)
		if err != nil {
			for previous := 0; previous < index; previous++ {
				_ = endpoints[previous].listener.Close()
			}
			return nil, fmt.Errorf("bind %s admin endpoint %s: %w", endpoint.name, endpoint.server.Addr, err)
		}
		endpoint.listener = listener
		endpoint.server.Addr = listener.Addr().String()
	}

	servers := make([]*http.Server, 0, len(endpoints))
	for index := range endpoints {
		endpoint := &endpoints[index]
		servers = append(servers, endpoint.server)
		go serveAdmin(logger, endpoint.server, endpoint.listener, endpoint.name, endpoint.path)
	}
	return servers, nil
}

func serveAdmin(logger *slog.Logger, server *http.Server, listener net.Listener, name, path string) {
	logger.Info("admin server started", "name", name, "addr", server.Addr, "path", path)
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		logger.Error("serve admin endpoint", "name", name, "addr", server.Addr, "error", err)
	}
}

func runDataCommand(ctx context.Context, cfg config.Config, args []string) error {
	if args[0] == "data-verify" || args[0] == "data-verify-media" {
		db, err := platform.OpenDatabaseReadOnly(cfg.DatabasePath)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := platform.VerifyDatabase(ctx, db); err != nil {
			return err
		}
		if args[0] == "data-verify-media" {
			if err := platform.VerifyMediaFiles(ctx, db, cfg.MediaDir); err != nil {
				return err
			}
		}
		stats, err := platform.ReadDatabaseStats(ctx, db)
		if err != nil {
			return err
		}
		fmt.Printf("integrity=ok\nmedia=%s\nusers=%d\nsessions=%d\nvideos=%d\nsubtitles=%d\ncomments=%d\nlikes=%d\nfavorites=%d\nfollows=%d\nmedia_jobs=%d\nnotifications=%d\n",
			map[bool]string{true: "ok", false: "not-checked"}[args[0] == "data-verify-media"],
			stats.Users, stats.Sessions, stats.Videos, stats.Subtitles, stats.Comments, stats.Likes, stats.Favorites, stats.Follows, stats.MediaJobs, stats.Notifications)
		return nil
	}

	db, err := platform.OpenDatabase(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()

	switch args[0] {
	case "data-status":
		stats, err := platform.ReadDatabaseStats(ctx, db)
		if err != nil {
			return err
		}
		fmt.Printf("database=%s\nusers=%d\nsessions=%d\nvideos=%d\nsubtitles=%d\ncomments=%d\nlikes=%d\nfavorites=%d\nfollows=%d\nmedia_jobs=%d\nnotifications=%d\n",
			cfg.DatabasePath, stats.Users, stats.Sessions, stats.Videos, stats.Subtitles, stats.Comments, stats.Likes, stats.Favorites, stats.Follows, stats.MediaJobs, stats.Notifications)
		return nil
	case "data-backup":
		if len(args) != 2 {
			return fmt.Errorf("usage: gvideo data-backup <destination.db>")
		}
		if err := platform.BackupDatabase(ctx, db, args[1]); err != nil {
			return err
		}
		fmt.Printf("backup=%s\n", args[1])
		return nil
	case "data-merge":
		if len(args) != 2 {
			return fmt.Errorf("usage: gvideo data-merge <source.db>")
		}
		stats, err := platform.MergeDatabase(ctx, db, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("users_added=%d\nusers_renamed=%d\nsessions_added=%d\nvideos_added=%d\ncomments_added=%d\nlikes_added=%d\nfavorites_added=%d\nfollows_added=%d\nmedia_jobs_added=%d\nnotifications_added=%d\n",
			stats.Users, stats.RenamedUsers, stats.Sessions, stats.Videos, stats.Comments, stats.Likes, stats.Favorites, stats.Follows, stats.MediaJobs, stats.Notifications)
		return nil
	case "data-grant-admin":
		if len(args) != 2 {
			return fmt.Errorf("usage: gvideo data-grant-admin <username>")
		}
		repo := repository.New(db)
		user, _, err := repo.UserAuthByUsername(ctx, strings.TrimSpace(args[1]))
		if err != nil {
			return err
		}
		if err := repo.SetAdminFlag(ctx, user.ID, true); err != nil {
			return fmt.Errorf("grant admin to %q: %w", user.Username, err)
		}
		fmt.Printf("admin_granted=%s\n", user.Username)
		return nil
	default:
		return fmt.Errorf("unknown command %q; use data-status, data-backup, data-verify, data-verify-media, data-merge, or data-grant-admin", args[0])
	}
}
