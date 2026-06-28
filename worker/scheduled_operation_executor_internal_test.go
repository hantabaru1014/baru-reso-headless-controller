package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
	_ "github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/actions"
	_ "github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/triggers"
	"github.com/stretchr/testify/assert"
)

// stubScheduledRepo は ScheduledOperationExecutor.executeOne の unit テスト用.
type stubScheduledRepo struct {
	port.ScheduledSessionOperationRepository

	failedID  string
	failedMsg string
	succeeded bool
}

func (r *stubScheduledRepo) MarkFailed(_ context.Context, id, msg string) error {
	r.failedID = id
	r.failedMsg = msg

	return nil
}

func (r *stubScheduledRepo) MarkSucceeded(_ context.Context, _ string) error {
	r.succeeded = true

	return nil
}

func (r *stubScheduledRepo) Requeue(_ context.Context, _ string, _ time.Time) error {
	return nil
}

// scheduledSessionOpStub は scheduled_op.SessionOperator の test stub.
type scheduledSessionOpStub struct {
	stopErr error

	gotCtx context.Context //nolint:containedctx // test stub: capture ctx for assertion
}

func (s *scheduledSessionOpStub) StartSession(_ context.Context, _, _ string, _ *string, _ *headlessv1.WorldStartupParameters, _ *string) (*entity.Session, error) {
	return nil, nil
}

func (s *scheduledSessionOpStub) StopSession(ctx context.Context, _ string) error {
	s.gotCtx = ctx

	return s.stopErr
}

func (s *scheduledSessionOpStub) UpdateSessionParameters(_ context.Context, _ string, _ *headlessv1.UpdateSessionParametersRequest) error {
	return nil
}

func (s *scheduledSessionOpStub) UpdateSessionExtraSettings(_ context.Context, _ string, _ *bool, _ *string) error {
	return nil
}

func newScheduledExecutor(repo port.ScheduledSessionOperationRepository, checker UserExistenceChecker, sessOp scheduled_op.SessionOperator) *ScheduledOperationExecutor {
	return NewScheduledOperationExecutor(repo, sessOp, nil, nil, checker, ScheduledOperationExecutorOptions{})
}

// newStopSessionOp は STOP_SESSION + TIME trigger (期日 1 時間前) の op を作る.
func newStopSessionOp(createdBy *string) *entity.ScheduledSessionOperation {
	payload, _ := json.Marshal(map[string]string{"session_id": "S-1"})                                                            //nolint:errchkjson // test fixture
	trigCfg, _ := json.Marshal(map[string]string{"scheduled_at": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)}) //nolint:errchkjson // test fixture

	return &entity.ScheduledSessionOperation{
		ID:               "op-1",
		OperationType:    entity.ScheduledOperationType_STOP_SESSION,
		OperationPayload: payload,
		TriggerType:      entity.ScheduledTriggerType_TIME,
		TriggerConfig:    trigCfg,
		NextFireAt:       time.Now().Add(-time.Hour),
		Status:           entity.ScheduledOperationStatus_RUNNING,
		CreatedBy:        createdBy,
	}
}

func strPtrSched(s string) *string { return &s }

func TestScheduledOperationExecutor_ExecuteOne_NoCreatedBy_MarksFailed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		createdBy *string
	}{
		{"nil createdBy", nil},
		{"empty createdBy", strPtrSched("")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &stubScheduledRepo{}
			sessOp := &scheduledSessionOpStub{}
			exe := newScheduledExecutor(repo, &stubUserChecker{exists: true}, sessOp)

			exe.executeOne(context.Background(), newStopSessionOp(tc.createdBy))

			assert.Equal(t, "op-1", repo.failedID)
			assert.Contains(t, repo.failedMsg, "scheduled operation has no created_by")
			assert.False(t, repo.succeeded)
			assert.Nil(t, sessOp.gotCtx, "action must not be invoked when created_by is missing")
		})
	}
}

func TestScheduledOperationExecutor_ExecuteOne_UserNotFound_MarksFailed(t *testing.T) {
	t.Parallel()

	repo := &stubScheduledRepo{}
	sessOp := &scheduledSessionOpStub{}
	exe := newScheduledExecutor(repo, &stubUserChecker{exists: false}, sessOp)

	exe.executeOne(context.Background(), newStopSessionOp(strPtrSched("ghost-user")))

	assert.Equal(t, "op-1", repo.failedID)
	assert.Contains(t, repo.failedMsg, "ghost-user")
	assert.Contains(t, repo.failedMsg, "does not exist")
	assert.False(t, repo.succeeded)
	assert.Nil(t, sessOp.gotCtx, "action must not be invoked when user is missing")
}

func TestScheduledOperationExecutor_ExecuteOne_PermissionDenied_MarksFailed(t *testing.T) {
	t.Parallel()

	repo := &stubScheduledRepo{}
	sessOp := &scheduledSessionOpStub{stopErr: domain.ErrPermissionDenied}
	exe := newScheduledExecutor(repo, &stubUserChecker{exists: true}, sessOp)

	exe.executeOne(context.Background(), newStopSessionOp(strPtrSched("user-A")))

	assert.Equal(t, "op-1", repo.failedID)
	assert.Contains(t, repo.failedMsg, domain.ErrPermissionDenied.Error())
	assert.False(t, repo.succeeded)
}

func TestScheduledOperationExecutor_ExecuteOne_SetsActAsUserCtx(t *testing.T) {
	t.Parallel()

	repo := &stubScheduledRepo{}
	sessOp := &scheduledSessionOpStub{}
	exe := newScheduledExecutor(repo, &stubUserChecker{exists: true}, sessOp)

	exe.executeOne(context.Background(), newStopSessionOp(strPtrSched("user-A")))

	if sessOp.gotCtx == nil {
		t.Fatalf("session operator was not invoked")
	}

	claims, err := auth.GetAuthClaimsFromContext(sessOp.gotCtx)
	if err != nil {
		t.Fatalf("expected AuthClaims in dispatched ctx, got error: %v", err)
	}

	assert.Equal(t, "user-A", claims.UserID)
	assert.True(t, repo.succeeded, "successful stop should mark op succeeded")
}
