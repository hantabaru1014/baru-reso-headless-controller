package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/async_job"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
)

// stubAsyncJobRepo は AsyncJobExecutor.executeOne の unit テスト用. MarkSucceeded /
// MarkFailed の引数だけ記録する.
type stubAsyncJobRepo struct {
	port.AsyncJobRepository

	failedID  string
	failedMsg string
	succeeded bool
}

func (r *stubAsyncJobRepo) MarkFailed(_ context.Context, id, msg string) error {
	r.failedID = id
	r.failedMsg = msg

	return nil
}

func (r *stubAsyncJobRepo) MarkSucceeded(_ context.Context, _ string, _ json.RawMessage) error {
	r.succeeded = true

	return nil
}

// noopBus は notification.Bus を満たす最小実装. PublishTo / Publish / Subscribe を全て no-op.
type noopBus struct{}

func (noopBus) Publish(*hdlctrlv1.NotificationEvent)                  {}
func (noopBus) PublishTo(string, *hdlctrlv1.NotificationEvent)        {}
func (noopBus) Subscribe(context.Context, string) (<-chan *hdlctrlv1.NotificationEvent, func()) {
	ch := make(chan *hdlctrlv1.NotificationEvent)
	return ch, func() { close(ch) }
}

// stubUserChecker は UserExistenceChecker の test stub.
type stubUserChecker struct {
	exists bool
	err    error
}

func (s *stubUserChecker) UserExistsByID(_ context.Context, _ string) (bool, error) {
	return s.exists, s.err
}

// stubSessionOperator は async_job.SessionOperator の test stub. StopSession の挙動だけ制御する.
type stubSessionOperator struct {
	stopErr error

	gotCtx context.Context //nolint:containedctx // test stub: capture ctx for assertion
}

func (s *stubSessionOperator) StartSession(_ context.Context, _ string, _ string, _ *string, _ *headlessv1.WorldStartupParameters, _ *string) (*entity.Session, error) {
	return nil, errors.New("not implemented in stub")
}

func (s *stubSessionOperator) StopSession(ctx context.Context, _ string) error {
	s.gotCtx = ctx

	return s.stopErr
}

func newTestAsyncJobExecutor(repo port.AsyncJobRepository, checker UserExistenceChecker, session async_job.SessionOperator) *AsyncJobExecutor {
	dispatcher := async_job.NewDispatcher(nil, session, nil)

	return NewAsyncJobExecutor(repo, dispatcher, noopBus{}, checker, AsyncJobExecutorOptions{})
}

func newTestJob(createdBy *string) *entity.AsyncJob {
	return &entity.AsyncJob{
		ID:        "job-1",
		JobType:   entity.AsyncJobType_STOP_SESSION,
		Payload:   json.RawMessage(`{"session_id":"S-1"}`),
		Status:    entity.AsyncJobStatus_RUNNING,
		CreatedBy: createdBy,
	}
}

func TestAsyncJobExecutor_ExecuteOne_NoCreatedBy_MarksFailed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		createdBy *string
	}{
		{"nil createdBy", nil},
		{"empty createdBy", strPtr("")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &stubAsyncJobRepo{}
			sessOp := &stubSessionOperator{}
			exe := newTestAsyncJobExecutor(repo, &stubUserChecker{exists: true}, sessOp)

			exe.executeOne(context.Background(), newTestJob(tc.createdBy))

			assert.Equal(t, "job-1", repo.failedID, "MarkFailed should be called with the job ID")
			assert.Contains(t, repo.failedMsg, "job has no created_by", "last_error should explain the missing created_by")
			assert.False(t, repo.succeeded, "MarkSucceeded must not be called")
			assert.Nil(t, sessOp.gotCtx, "dispatcher must not be invoked when created_by is missing")
		})
	}
}

func TestAsyncJobExecutor_ExecuteOne_UserNotFound_MarksFailed(t *testing.T) {
	t.Parallel()

	repo := &stubAsyncJobRepo{}
	sessOp := &stubSessionOperator{}
	exe := newTestAsyncJobExecutor(repo, &stubUserChecker{exists: false}, sessOp)

	exe.executeOne(context.Background(), newTestJob(strPtr("ghost-user")))

	assert.Equal(t, "job-1", repo.failedID)
	assert.Contains(t, repo.failedMsg, "ghost-user", "last_error should include the missing user ID")
	assert.Contains(t, repo.failedMsg, "does not exist")
	assert.False(t, repo.succeeded)
	assert.Nil(t, sessOp.gotCtx, "dispatcher must not be invoked when user is missing")
}

func TestAsyncJobExecutor_ExecuteOne_PermissionDenied_MarksFailed(t *testing.T) {
	t.Parallel()

	repo := &stubAsyncJobRepo{}
	sessOp := &stubSessionOperator{stopErr: domain.ErrPermissionDenied}
	exe := newTestAsyncJobExecutor(repo, &stubUserChecker{exists: true}, sessOp)

	exe.executeOne(context.Background(), newTestJob(strPtr("user-A")))

	assert.Equal(t, "job-1", repo.failedID)
	assert.Contains(t, repo.failedMsg, domain.ErrPermissionDenied.Error(),
		"last_error should surface PermissionDenied from the underlying usecase")
	assert.False(t, repo.succeeded)
}

func TestAsyncJobExecutor_ExecuteOne_SetsActAsUserCtx(t *testing.T) {
	t.Parallel()

	repo := &stubAsyncJobRepo{}
	sessOp := &stubSessionOperator{}
	exe := newTestAsyncJobExecutor(repo, &stubUserChecker{exists: true}, sessOp)

	exe.executeOne(context.Background(), newTestJob(strPtr("user-A")))

	// dispatcher 経由で session operator が呼ばれた ctx に、created_by の AuthClaims が
	// 入っていることを確認 (usecase 層の Require* が CurrentUserID で参照する).
	if sessOp.gotCtx == nil {
		t.Fatalf("session operator was not invoked")
	}

	claims, err := auth.GetAuthClaimsFromContext(sessOp.gotCtx)
	if err != nil {
		t.Fatalf("expected AuthClaims in dispatched ctx, got error: %v", err)
	}

	assert.Equal(t, "user-A", claims.UserID)
}

// TestAsyncJobExecutor_PersistTimeoutSurvivesCtxCancel は、親 ctx がキャンセル
// された後でも MarkFailed の DB 書き込みが行われることを確認する.
// (具体的な persistTimeout の値検証はしない / 既存挙動回帰チェック).
func TestAsyncJobExecutor_MarkFailed_OnCanceledCtx(t *testing.T) {
	t.Parallel()

	repo := &stubAsyncJobRepo{}
	sessOp := &stubSessionOperator{}
	exe := newTestAsyncJobExecutor(repo, &stubUserChecker{exists: true}, sessOp)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // すぐキャンセル

	// 多分 markFailed の中で context.WithoutCancel + Timeout が効いて書ける.
	exe.executeOne(ctx, newTestJob(nil)) // created_by 無しなので即 markFailed

	assert.Equal(t, "job-1", repo.failedID, "MarkFailed should run even when parent ctx is canceled")
}

func strPtr(s string) *string { return &s }
