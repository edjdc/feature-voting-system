package ranking

import (
	"context"
	"log/slog"
	"time"
)

type ReconcileStore interface {
	GetVoteCountMismatches(ctx context.Context) ([]struct {
		ID            string
		StoredCount   int32
		ActualCount   int32
	}, error)
	FixVoteCount(ctx context.Context, requestID string, count int32) error
	SetCachedCount(ctx context.Context, requestID string, count int32)
}

type ReconcileJob struct {
	store    ReconcileStore
	interval time.Duration
	log      *slog.Logger
}

func NewReconcileJob(store ReconcileStore, interval time.Duration, log *slog.Logger) *ReconcileJob {
	return &ReconcileJob{store: store, interval: interval, log: log}
}

// Run starts the periodic reconciliation job. Stops when ctx is cancelled.
func (j *ReconcileJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			j.log.Info("reconcile job stopping")
			return
		case <-ticker.C:
			if err := j.reconcile(ctx); err != nil {
				j.log.Error("reconcile failed", "error", err)
			}
		}
	}
}

func (j *ReconcileJob) reconcile(ctx context.Context) error {
	mismatches, err := j.store.GetVoteCountMismatches(ctx)
	if err != nil {
		return err
	}

	for _, m := range mismatches {
		j.log.Warn("vote_count mismatch detected",
			"request_id", m.ID,
			"stored", m.StoredCount,
			"actual", m.ActualCount,
		)
		if err := j.store.FixVoteCount(ctx, m.ID, m.ActualCount); err != nil {
			j.log.Error("fix vote count", "request_id", m.ID, "error", err)
			continue
		}
		j.store.SetCachedCount(ctx, m.ID, m.ActualCount)
	}
	return nil
}
