package worker

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
)

// ScheduledOperationExecutor periodically claims due scheduled session operations,
// evaluates their triggers, and dispatches the action via the registered
// scheduled_op factories. Concurrency-safe across multiple server instances
// via FOR UPDATE SKIP LOCKED at the repository layer.
type ScheduledOperationExecutor struct {
	repo        port.ScheduledSessionOperationRepository
	sessionOp   scheduled_op.SessionOperator
	sessionRepo port.SessionRepository
	stateCache  port.SessionStateCache

	instanceID     string
	tickInterval   time.Duration
	staleAfter     time.Duration
	staleSweepEvry time.Duration
	batchSize      int32
	concurrency    int
}

const (
	defaultExecutorTickInterval  = 10 * time.Second
	defaultExecutorStaleAfter    = 10 * time.Minute
	defaultExecutorStaleSweep    = 1 * time.Minute
	defaultExecutorBatchSize     = 16
	defaultExecutorConcurrency   = 4
	defaultExecutorActionTimeout = 2 * time.Minute
)

type ScheduledOperationExecutorOptions struct {
	InstanceID    string
	TickInterval  time.Duration
	StaleAfter    time.Duration
	StaleSweep    time.Duration
	BatchSize     int32
	Concurrency   int
	ActionTimeout time.Duration
}

func NewScheduledOperationExecutor(
	repo port.ScheduledSessionOperationRepository,
	sessionOp scheduled_op.SessionOperator,
	sessionRepo port.SessionRepository,
	stateCache port.SessionStateCache,
	opts ScheduledOperationExecutorOptions,
) *ScheduledOperationExecutor {
	if opts.InstanceID == "" {
		hostname, err := os.Hostname()
		if err == nil && hostname != "" {
			opts.InstanceID = hostname
		} else {
			opts.InstanceID = "controller"
		}
	}

	if opts.TickInterval <= 0 {
		opts.TickInterval = defaultExecutorTickInterval
	}

	if opts.StaleAfter <= 0 {
		opts.StaleAfter = defaultExecutorStaleAfter
	}

	if opts.StaleSweep <= 0 {
		opts.StaleSweep = defaultExecutorStaleSweep
	}

	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultExecutorBatchSize
	}

	if opts.Concurrency <= 0 {
		opts.Concurrency = defaultExecutorConcurrency
	}

	return &ScheduledOperationExecutor{
		repo:           repo,
		sessionOp:      sessionOp,
		sessionRepo:    sessionRepo,
		stateCache:     stateCache,
		instanceID:     opts.InstanceID,
		tickInterval:   opts.TickInterval,
		staleAfter:     opts.StaleAfter,
		staleSweepEvry: opts.StaleSweep,
		batchSize:      opts.BatchSize,
		concurrency:    opts.Concurrency,
	}
}

var _ Runner = (*ScheduledOperationExecutor)(nil)

func (e *ScheduledOperationExecutor) Name() string { return "scheduled-operation-executor" }

func (e *ScheduledOperationExecutor) Run(ctx context.Context) error {
	// 起動時 sweep: 前回プロセスが落ちて RUNNING のまま残った行を救う.
	if rows, err := e.repo.ReleaseStaleClaims(ctx, e.staleAfter); err != nil {
		slog.Warn("scheduled-operation-executor: initial stale claim sweep failed", "error", err)
	} else if rows > 0 {
		slog.Info("scheduled-operation-executor: released stale claims at startup", "rows", rows)
	}

	tick := time.NewTicker(e.tickInterval)
	defer tick.Stop()

	stale := time.NewTicker(e.staleSweepEvry)
	defer stale.Stop()

	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()

			return ctx.Err()
		case <-stale.C:
			if rows, err := e.repo.ReleaseStaleClaims(ctx, e.staleAfter); err != nil {
				slog.Warn("scheduled-operation-executor: stale claim sweep failed", "error", err)
			} else if rows > 0 {
				slog.Info("scheduled-operation-executor: released stale claims", "rows", rows)
			}
		case <-tick.C:
			e.dispatchOnce(ctx, &wg)
		}
	}
}

func (e *ScheduledOperationExecutor) dispatchOnce(ctx context.Context, wg *sync.WaitGroup) {
	ops, err := e.repo.ClaimDue(ctx, e.instanceID, e.batchSize)
	if err != nil {
		slog.Warn("scheduled-operation-executor: claim failed", "error", err)
		return
	}

	if len(ops) == 0 {
		return
	}

	sem := make(chan struct{}, e.concurrency)

	for _, op := range ops {
		select {
		case <-ctx.Done():
			// shutdown 中. claim 済みは stale sweep か次回 startup で復旧する.
			return
		case sem <- struct{}{}:
		}

		wg.Go(func() {
			defer func() { <-sem }()

			e.executeOne(ctx, op)
		})
	}
}

func (e *ScheduledOperationExecutor) executeOne(ctx context.Context, op *entity.ScheduledSessionOperation) {
	logger := slog.With("op_id", op.ID, "type", op.OperationType, "trigger", op.TriggerType)

	trig, err := scheduled_op.DecodeTrigger(op.TriggerType, op.TriggerConfig)
	if err != nil {
		logger.Error("scheduled-operation-executor: decode trigger failed", "error", err)
		e.markFailed(ctx, op.ID, errors.WrapPrefix(err, "decode trigger", 0))

		return
	}

	ready, nextCheck, err := trig.Evaluate(ctx, scheduled_op.TriggerEvalDeps{
		Now:         time.Now,
		SessionRepo: e.sessionRepo,
		StateCache:  e.stateCache,
	})
	if err != nil {
		logger.Error("scheduled-operation-executor: evaluate trigger failed", "error", err)
		e.markFailed(ctx, op.ID, errors.WrapPrefix(err, "evaluate trigger", 0))

		return
	}

	if !ready {
		// 未だ ready ではない (condition 系 trigger 用)。PENDING に戻し、再評価時刻を更新.
		if nextCheck.IsZero() {
			nextCheck = time.Now().Add(e.tickInterval)
		}

		if err := e.repo.Requeue(ctx, op.ID, nextCheck); err != nil {
			logger.Error("scheduled-operation-executor: requeue failed", "error", err)
		}

		return
	}

	act, err := scheduled_op.DecodeAction(op.OperationType, op.OperationPayload)
	if err != nil {
		logger.Error("scheduled-operation-executor: decode action failed", "error", err)
		e.markFailed(ctx, op.ID, errors.WrapPrefix(err, "decode action", 0))

		return
	}

	actCtx, cancel := context.WithTimeout(ctx, defaultExecutorActionTimeout)
	defer cancel()

	if err := act.Execute(actCtx, scheduled_op.ActionExecDeps{Session: e.sessionOp}); err != nil {
		logger.Error("scheduled-operation-executor: execute failed", "error", err)
		e.markFailed(ctx, op.ID, err)

		return
	}

	if err := e.repo.MarkSucceeded(ctx, op.ID); err != nil {
		logger.Error("scheduled-operation-executor: mark succeeded failed", "error", err)
		return
	}

	logger.Info("scheduled-operation-executor: succeeded")
}

func (e *ScheduledOperationExecutor) markFailed(ctx context.Context, id string, runErr error) {
	msg := runErr.Error()
	if err := e.repo.MarkFailed(ctx, id, msg); err != nil {
		slog.Error("scheduled-operation-executor: mark failed errored", "op_id", id, "error", err)
	}
}
