package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/httpapi"
	"gvideo/backend/internal/media"
	"gvideo/backend/internal/platform"
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
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("storage initialized", "database_path", cfg.DatabasePath, "media_dir", cfg.MediaDir)

	repo := repository.New(db)
	svc := service.New(repo, cfg, logger)
	handler := httpapi.New(svc, cfg, logger)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      15 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.MediaWorkerEnabled {
		transcoder := media.NewFFmpegTranscoder(cfg.HLSEnabled, cfg.FFmpegPath, cfg.MediaDir, cfg.HLSTranscodeTimeout, cfg.HLSSegmentSeconds)
		worker := media.NewWorker(repo, media.NewFFprobe(cfg.FFprobePath, cfg.MediaProbeTimeout), transcoder, cfg.MediaDir, cfg.MediaWorkerPollInterval, logger)
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
}

func runDataCommand(ctx context.Context, cfg config.Config, args []string) error {
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
		fmt.Printf("database=%s\nusers=%d\nsessions=%d\nvideos=%d\ncomments=%d\nlikes=%d\nfavorites=%d\nfollows=%d\nmedia_jobs=%d\n",
			cfg.DatabasePath, stats.Users, stats.Sessions, stats.Videos, stats.Comments, stats.Likes, stats.Favorites, stats.Follows, stats.MediaJobs)
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
		fmt.Printf("users_added=%d\nusers_renamed=%d\nsessions_added=%d\nvideos_added=%d\ncomments_added=%d\nlikes_added=%d\nfavorites_added=%d\nfollows_added=%d\nmedia_jobs_added=%d\n",
			stats.Users, stats.RenamedUsers, stats.Sessions, stats.Videos, stats.Comments, stats.Likes, stats.Favorites, stats.Follows, stats.MediaJobs)
		return nil
	default:
		return fmt.Errorf("unknown command %q; use data-status, data-backup, or data-merge", args[0])
	}
}
