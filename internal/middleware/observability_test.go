package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"github.com/edivilsondalacosta/feature-voting-system/internal/middleware"
	"github.com/edivilsondalacosta/feature-voting-system/internal/observability"
)

func TestRecover_NoPanic(t *testing.T) {
	log := observability.NewLogger()
	h := middleware.Recover(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRecover_Panic(t *testing.T) {
	log := observability.NewLogger()
	h := middleware.Recover(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRequestLogging_PassesThrough(t *testing.T) {
	log := observability.NewLogger()
	h := middleware.RequestLogging(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodPost, "/items", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestMetrics_RecordsRequests(t *testing.T) {
	m := observability.NewMetrics(prometheus.NewRegistry())

	h := middleware.Metrics(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}
