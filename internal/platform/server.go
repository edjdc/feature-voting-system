package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	srv *http.Server
	log *slog.Logger
}

func NewServer(port string, handler http.Handler, log *slog.Logger) *Server {
	return &Server{
		srv: &http.Server{
			Addr:         ":" + port,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		log: log,
	}
}

func (s *Server) ListenAndServe() error {
	s.log.Info("server starting", "addr", s.srv.Addr)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("server shutting down")
	return s.srv.Shutdown(ctx)
}
