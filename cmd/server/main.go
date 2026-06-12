package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/edivilsondalacosta/feature-voting-system/internal/config"
	"github.com/edivilsondalacosta/feature-voting-system/internal/handler"
	"github.com/edivilsondalacosta/feature-voting-system/internal/middleware"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
	"github.com/edivilsondalacosta/feature-voting-system/internal/platform"
	pgRepo "github.com/edivilsondalacosta/feature-voting-system/internal/repository/postgres"
	"github.com/edivilsondalacosta/feature-voting-system/internal/ranking"
	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

func main() {
	log := observability.NewLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// OTel tracer
	_, shutdownTracer, err := observability.NewTracer(ctx, "feature-voting-system")
	if err != nil {
		log.Error("init tracer", "error", err)
		os.Exit(1)
	}
	defer shutdownTracer(ctx)

	// Prometheus metrics
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)

	// Postgres pool
	pool, err := pgRepo.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Redis client
	rdb, err := connectRedis(ctx, cfg.RedisURL, log)
	if err != nil {
		log.Error("connect redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// Repositories
	userRepo := pgRepo.NewUserRepo(pool)
	requestRepo := pgRepo.NewRequestRepo(pool)
	voteRepo := pgRepo.NewVoteRepo(pool)
	rankingPgRepo := pgRepo.NewRankingRepo(pool)

	// Services
	authSvc := service.NewAuthService(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	userSvc := service.NewUserService(userRepo, authSvc, log)
	requestSvc := service.NewRequestService(requestRepo, log)
	voteSvc := service.NewVoteService(voteRepo, log)
	rankingSvc := service.NewRankingService(requestRepo, log)

	// Background jobs
	trendingStore := &trendingStoreAdapter{pg: rankingPgRepo, rdb: rdb}
	trendingJob := ranking.NewTrendingJob(trendingStore, cfg.TrendingRecomputeInterval, log)
	go trendingJob.Run(ctx)

	reconcileStore := &reconcileStoreAdapter{pg: rankingPgRepo, rdb: rdb}
	reconcileJob := ranking.NewReconcileJob(reconcileStore, 5*time.Minute, log)
	go reconcileJob.Run(ctx)

	// Handlers
	authH := handler.NewAuthHandler(authSvc, userSvc, log)
	requestsH := handler.NewRequestsHandler(requestSvc, rankingSvc, log)
	votesH := handler.NewVotesHandler(voteSvc, log)

	// Router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(middleware.Recover(log))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.RequestLogging(log))
	r.Use(middleware.Metrics(metrics))
	r.Use(middleware.OTel("feature-voting-system"))

	// Ops routes (no auth)
	r.Get("/healthz", platform.Healthz)
	r.Get("/health", platform.Health)
	r.Get("/readyz", platform.Readyz(pool, rdb))
	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	// Auth routes
	r.Post("/auth/register", authH.Register)
	r.Post("/auth/login", authH.Login)
	r.Post("/auth/refresh", authH.Refresh)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authSvc))
		r.Use(middleware.RateLimit(rdb, 100, time.Minute))

		r.Post("/requests", requestsH.Submit)
		r.Get("/requests", requestsH.List)
		r.Get("/requests/{id}", requestsH.GetByID)

		r.Put("/requests/{id}/vote", votesH.Upvote)
		r.Delete("/requests/{id}/vote", votesH.RemoveVote)
	})

	// Server
	srv := platform.NewServer(cfg.Port, r, log)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error("server error", "error", err)
		}
	}()

	<-quit
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}
	log.Info("server stopped")
}
