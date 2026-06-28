package worker

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
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
	userChecker UserExistenceChecker

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
	userChecker UserExistenceChecker,
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
		userChecker:    userChecker,
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

	// created_by が無い / DB 上のユーザーが消えている場合は実行主体を立てられないので
	// 即 FAILED にする. systemPrivilege fallback はしない (作成権限の追跡を維持するため).
	if op.CreatedBy == nil || *op.CreatedBy == "" {
		logger.Error("scheduled-operation-executor: operation has no created_by; marking failed")
		e.markFailed(ctx, op.ID, errors.New("scheduled operation has no created_by"))

		return
	}

	if e.userChecker != nil {
		exists, err := e.userChecker.UserExistsByID(ctx, *op.CreatedBy)
		if err != nil {
			// ctx キャンセル (worker shutdown 等) は transient. 永続的に FAILED 化
			// せず claim を残し、stale sweep でリトライ可能にする.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				logger.Warn("scheduled-operation-executor: check created_by user canceled; will retry", "error", err)
				return
			}

			logger.Error("scheduled-operation-executor: check created_by user failed", "error", err)
			e.markFailed(ctx, op.ID, errors.WrapPrefix(err, "check created_by user", 0))

			return
		}

		if !exists {
			logger.Error("scheduled-operation-executor: created_by user does not exist; marking failed", "user_id", *op.CreatedBy)
			e.markFailed(ctx, op.ID, errors.Errorf("created_by user %q does not exist", *op.CreatedBy))

			return
		}
	}

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

	// 以降の usecase 呼び出しは created_by を実行主体とする ctx で行う.
	// 権限剥奪後の op 実行は usecase 層の Require* で PermissionDenied になり
	// 自然に FAILED に倒れる.
	actCtx = auth.WithActAsUser(actCtx, *op.CreatedBy)

	if err := act.Execute(actCtx, scheduled_op.ActionExecDeps{Session: e.sessionOp}); err != nil {
		logger.Error("scheduled-operation-executor: execute failed", "error", err)
		e.markFailed(ctx, op.ID, err)

		return
	}

	// 永続化は worker shutdown の ctx cancel と独立に行う. 親 ctx が cancel された場合に
	// MarkSucceeded が早期失敗すると op は RUNNING のまま残り、stale sweep で再実行されると
	// StopSession 等が冪等でも UpdateParameters 等は副作用が重複する.
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer persistCancel()

	if err := e.repo.MarkSucceeded(persistCtx, op.ID); err != nil {
		logger.Error("scheduled-operation-executor: mark succeeded failed", "error", err)
		return
	}

	logger.Info("scheduled-operation-executor: succeeded")
}

func (e *ScheduledOperationExecutor) markFailed(ctx context.Context, id string, runErr error) {
	msg := runErr.Error()

	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()

	if err := e.repo.MarkFailed(persistCtx, id, msg); err != nil {
		slog.Error("scheduled-operation-executor: mark failed errored", "op_id", id, "error", err)
	}
}
