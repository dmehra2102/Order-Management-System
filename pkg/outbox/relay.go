package outbox

import (
	"context"
	"log/slog"
	"time"
)

type Store interface {
	LockBatch(ctx context.Context, relayID string, batchSize int, lease time.Duration) ([]Event, error)
	MarkSent(ctx context.Context, ids []int64) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
	ExtendLease(ctx context.Context, relayID string, ids []int64, lease time.Duration) error
}

type Relay struct {
	log       *slog.Logger
	store     Store
	dispatch  *Dispatcher
	relayID   string
	batchSize int
	interval  time.Duration
	lease     time.Duration
}

func NewRelay(log *slog.Logger, store Store, dispatch *Dispatcher, relayID string) *Relay {
	return &Relay{
		log:       log,
		store:     store,
		dispatch:  dispatch,
		relayID:   relayID,
		batchSize: 100,
		interval:  500 * time.Millisecond,
		lease:     5 * time.Second,
	}
}

func (r *Relay) Run(ctx context.Context) error {
	t := time.NewTicker(r.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info("relay stopping", "relay_id", r.relayID)
			return nil
		case <-t.C:
			events, err := r.store.LockBatch(ctx, r.relayID, r.batchSize, r.lease)
			if err != nil {
				r.log.Error("relay lock batch error", "err", err)
				continue
			}
			if len(events) == 0 {
				continue
			}

			ids := make([]int64, 0, len(events))
			for _, e := range events {
				if err := r.dispatch.Disptach(ctx, e); err != nil {
					_ = r.store.MarkFailed(ctx, e.ID, err.Error())
					continue
				}
				ids = append(ids, e.ID)
			}
			if len(ids) > 0 {
				if err := r.store.MarkSent(ctx, ids); err != nil {
					r.log.Error("relay mark sent error", "err", err)
				}
			}
		}
	}
}
