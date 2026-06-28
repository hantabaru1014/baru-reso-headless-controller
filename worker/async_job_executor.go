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
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/async_job"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/notification"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// UserExistenceChecker は job 実行前に created_by ユーザーが存在することを
// 確認するための narrow interface. *db.Queries の GetUser を薄くラップする
// 実装を adapter 側で提供する想定 (NewQueriesUserExistenceChecker).
type UserExistenceChecker interface {
	UserExistsByID(ctx context.Context, userID string) (bool, error)
}

// AsyncJobExecutor は非同期 job (ホスト/セッションの起動・停止系) を回す worker.
// ScheduledOperationExecutor と同じ FOR UPDATE SKIP LOCKED claim パターンで
// マルチインスタンス安全を担保し、stale claim sweep でクラッシュした instance を救う.
//
// 完了時には notification.Bus.PublishTo(createdBy, JobCompletedEvent) で
// 投入元の user にだけ通知を push する.
type AsyncJobExecutor struct {
	repo        port.AsyncJobRepository
	dispatcher  *async_job.Dispatcher
	bus         notification.Bus
	userChecker UserExistenceChecker

	instanceID     string
	tickInterval   time.Duration
	staleAfter     time.Duration
	staleSweepEvry time.Duration
	batchSize      int32
	concurrency    int
	actionTimeout  time.Duration
}

const (
	// 起動直後の job をなるべく早く拾うが、idle 時の DB 負荷も気にして 5 秒.
	defaultAsyncJobTickInterval  = 5 * time.Second
	defaultAsyncJobStaleAfter    = 30 * time.Minute
	defaultAsyncJobStaleSweep    = 1 * time.Minute
	defaultAsyncJobBatchSize     = 8
	defaultAsyncJobConcurrency   = 4
	// 個別 job の最大実行時間. StartHost は docker pull + container 起動 + RPC ハンドシェイクで
	// 数分かかりうるため長めに取る. ShutdownHost / StopSession はもっと短いが、
	// per-job-type timeout を入れる程の差分は今のところない.
	defaultAsyncJobActionTimeout = 20 * time.Minute
)

type AsyncJobExecutorOptions struct {
	InstanceID    string
	TickInterval  time.Duration
	StaleAfter    time.Duration
	StaleSweep    time.Duration
	BatchSize     int32
	Concurrency   int
	ActionTimeout time.Duration
}

func NewAsyncJobExecutor(
	repo port.AsyncJobRepository,
	dispatcher *async_job.Dispatcher,
	bus notification.Bus,
	userChecker UserExistenceChecker,
	opts AsyncJobExecutorOptions,
) *AsyncJobExecutor {
	if opts.InstanceID == "" {
		hostname, err := os.Hostname()
		if err == nil && hostname != "" {
			opts.InstanceID = hostname
		} else {
			opts.InstanceID = "controller"
		}
	}

	if opts.TickInterval <= 0 {
		opts.TickInterval = defaultAsyncJobTickInterval
	}

	if opts.StaleAfter <= 0 {
		opts.StaleAfter = defaultAsyncJobStaleAfter
	}

	if opts.StaleSweep <= 0 {
		opts.StaleSweep = defaultAsyncJobStaleSweep
	}

	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultAsyncJobBatchSize
	}

	if opts.Concurrency <= 0 {
		opts.Concurrency = defaultAsyncJobConcurrency
	}

	if opts.ActionTimeout <= 0 {
		opts.ActionTimeout = defaultAsyncJobActionTimeout
	}

	return &AsyncJobExecutor{
		repo:           repo,
		dispatcher:     dispatcher,
		bus:            bus,
		userChecker:    userChecker,
		instanceID:     opts.InstanceID,
		tickInterval:   opts.TickInterval,
		staleAfter:     opts.StaleAfter,
		staleSweepEvry: opts.StaleSweep,
		batchSize:      opts.BatchSize,
		concurrency:    opts.Concurrency,
		actionTimeout:  opts.ActionTimeout,
	}
}

var _ Runner = (*AsyncJobExecutor)(nil)

func (e *AsyncJobExecutor) Name() string { return "async-job-executor" }

func (e *AsyncJobExecutor) Run(ctx context.Context) error {
	if rows, err := e.repo.ReleaseStaleClaims(ctx, e.staleAfter); err != nil {
		slog.Warn("async-job-executor: initial stale claim sweep failed", "error", err)
	} else if rows > 0 {
		slog.Info("async-job-executor: released stale claims at startup", "rows", rows)
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
				slog.Warn("async-job-executor: stale claim sweep failed", "error", err)
			} else if rows > 0 {
				slog.Info("async-job-executor: released stale claims", "rows", rows)
			}
		case <-tick.C:
			e.dispatchOnce(ctx, &wg)
		}
	}
}

func (e *AsyncJobExecutor) dispatchOnce(ctx context.Context, wg *sync.WaitGroup) {
	jobs, err := e.repo.ClaimDue(ctx, e.instanceID, e.batchSize)
	if err != nil {
		slog.Warn("async-job-executor: claim failed", "error", err)
		return
	}

	if len(jobs) == 0 {
		return
	}

	sem := make(chan struct{}, e.concurrency)

	for _, j := range jobs {
		select {
		case <-ctx.Done():
			// shutdown 中. claim 済みは stale sweep か次回 startup で復旧する.
			return
		case sem <- struct{}{}:
		}

		wg.Go(func() {
			defer func() { <-sem }()

			e.executeOne(ctx, j)
		})
	}
}

// persistTimeout は MarkSucceeded / MarkFailed の DB 書き込みに与える上限時間.
// 短すぎると graceful shutdown 中に間に合わず job が RUNNING のまま残って
// stale sweep 経由で再実行されてしまう (StartHost は非冪等). 余裕を持って 10 秒.
const persistTimeout = 10 * time.Second

func (e *AsyncJobExecutor) executeOne(ctx context.Context, job *entity.AsyncJob) {
	logger := slog.With("job_id", job.ID, "type", job.JobType)

	// created_by が無い / DB 上のユーザーが消えている場合は実行主体を立てられないので
	// 即 FAILED にする. systemPrivilege fallback はしない (作成権限の追跡を維持するため).
	if job.CreatedBy == nil || *job.CreatedBy == "" {
		logger.Error("async-job-executor: job has no created_by; marking failed")
		e.markFailed(ctx, job, errors.New("job has no created_by"))

		return
	}

	if e.userChecker != nil {
		exists, err := e.userChecker.UserExistsByID(ctx, *job.CreatedBy)
		if err != nil {
			// ctx キャンセル (worker shutdown 等) は transient. 永続的に FAILED 化
			// せず claim を残し、stale sweep でリトライ可能にする.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				logger.Warn("async-job-executor: check created_by user canceled; will retry", "error", err)
				return
			}

			logger.Error("async-job-executor: check created_by user failed", "error", err)
			e.markFailed(ctx, job, errors.WrapPrefix(err, "check created_by user", 0))

			return
		}

		if !exists {
			logger.Error("async-job-executor: created_by user does not exist; marking failed", "user_id", *job.CreatedBy)
			e.markFailed(ctx, job, errors.Errorf("created_by user %q does not exist", *job.CreatedBy))

			return
		}
	}

	actCtx, cancel := context.WithTimeout(ctx, e.actionTimeout)
	defer cancel()

	// 以降の usecase 呼び出しは created_by を実行主体とする ctx で行う.
	// 権限剥奪後の job 実行は usecase 層の Require* で PermissionDenied になり
	// 自然に FAILED に倒れる.
	actCtx = auth.WithActAsUser(actCtx, *job.CreatedBy)

	defer func() {
		if rv := recover(); rv != nil {
			logger.Error("async-job-executor: handler panicked", "panic", rv)
			e.markFailed(ctx, job, errors.Errorf("handler panicked: %v", rv))
		}
	}()

	result, message, runErr := e.dispatcher.Dispatch(actCtx, job)
	if runErr != nil {
		logger.Error("async-job-executor: execute failed", "error", runErr)
		e.markFailed(ctx, job, runErr)

		return
	}

	resultJSON, marshalErr := async_job.MarshalResult(result)
	if marshalErr != nil {
		// result の marshal が失敗するのは普通起きないが、起きた場合は失敗扱いにしておく.
		logger.Error("async-job-executor: marshal result failed", "error", marshalErr)
		e.markFailed(ctx, job, marshalErr)

		return
	}

	// 永続化は worker shutdown の ctx cancel と独立に行う. 親 ctx が cancel された場合に
	// MarkSucceeded が早期失敗すると job は RUNNING のまま残り、後で stale sweep に
	// 再実行されてしまう (StartHost は非冪等で二重起動になる). detach + 上限のみ.
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer persistCancel()

	if err := e.repo.MarkSucceeded(persistCtx, job.ID, resultJSON); err != nil {
		// 永続化に失敗しても work は完了しているので通知は飛ばす.
		// markFailed と挙動を揃え、ユーザ向け toast が "ずっと出ない" 状態を作らない.
		logger.Error("async-job-executor: mark succeeded failed", "error", err)
	} else {
		logger.Info("async-job-executor: succeeded")
	}

	e.publishCompletion(job, message, hdlctrlv1.JobCompletedEvent_LEVEL_SUCCESS)
}

func (e *AsyncJobExecutor) markFailed(ctx context.Context, job *entity.AsyncJob, runErr error) {
	msg := runErr.Error()

	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()

	if err := e.repo.MarkFailed(persistCtx, job.ID, msg); err != nil {
		slog.Error("async-job-executor: mark failed errored", "job_id", job.ID, "error", err)
	}

	e.publishCompletion(job, msg, hdlctrlv1.JobCompletedEvent_LEVEL_ERROR)
}

func (e *AsyncJobExecutor) publishCompletion(job *entity.AsyncJob, message string, level hdlctrlv1.JobCompletedEvent_Level) {
	if job.CreatedBy == nil || *job.CreatedBy == "" {
		// system / CLI 投入の job (currently 該当無し) は宛先がないので broadcast せず捨てる.
		// 必要になったら bus.Publish (全配信) に切り替える.
		return
	}

	e.bus.PublishTo(*job.CreatedBy, notification.JobCompleted(job.ID, message, level))
}
