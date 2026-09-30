package usecase

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// startingHostRepo は起動途中のホストを模す stub. status と RPC client だけを返し、
// それ以外のメソッド (Restart など) は呼ばれたら panic する.
type startingHostRepo struct {
	port.HeadlessHostRepository

	status atomic.Int32
	client headlessv1.HeadlessControlServiceClient
}

func (r *startingHostRepo) GetStatus(_ context.Context, _ string) (entity.HeadlessHostStatus, error) {
	return entity.HeadlessHostStatus(r.status.Load()), nil
}

func (r *startingHostRepo) GetRpcClient(_ context.Context, _ string) (headlessv1.HeadlessControlServiceClient, error) {
	return r.client, nil
}

// bootingClient は readyAfter 回目の GetAccountInfo から成功する (それまではログイン前扱い).
type bootingClient struct {
	headlessv1.HeadlessControlServiceClient

	readyAfter int32
	probes     atomic.Int32
}

func (c *bootingClient) GetAccountInfo(_ context.Context, _ *headlessv1.GetAccountInfoRequest, _ ...grpc.CallOption) (*headlessv1.GetAccountInfoResponse, error) {
	if c.probes.Add(1) < c.readyAfter {
		return nil, errors.New("headless is not login")
	}

	return &headlessv1.GetAccountInfoResponse{}, nil
}

func newHostUsecaseUnderTest(repo port.HeadlessHostRepository) *HeadlessHostUsecase {
	hhuc := NewHeadlessHostUsecase(repo, nil, nil, nil, nil, nil)
	hhuc.hostReadyPollInterval = time.Millisecond

	return hhuc
}

func TestHeadlessHostEnsureRunning_LeavesNonStoppedHostAlone(t *testing.T) {
	t.Parallel()

	for _, status := range []entity.HeadlessHostStatus{
		entity.HeadlessHostStatus_RUNNING,
		entity.HeadlessHostStatus_STARTING,
		entity.HeadlessHostStatus_STOPPING,
	} {
		// Restart 系を実装していない stub なので、起動しようとすれば panic する.
		repo := &startingHostRepo{}
		repo.status.Store(int32(status))

		started, err := newHostUsecaseUnderTest(repo).HeadlessHostEnsureRunning(t.Context(), "h-1")
		require.NoError(t, err)
		assert.False(t, started)
	}
}

func TestWaitUntilReady(t *testing.T) {
	t.Parallel()

	t.Run("ログインが終わって RPC が通るまで待つ", func(t *testing.T) {
		t.Parallel()

		client := &bootingClient{readyAfter: 3}
		repo := &startingHostRepo{client: client}
		repo.status.Store(int32(entity.HeadlessHostStatus_RUNNING))

		require.NoError(t, newHostUsecaseUnderTest(repo).waitUntilReady(t.Context(), "h-1"))
		assert.Equal(t, int32(3), client.probes.Load())
	})

	t.Run("起動途中でコンテナが落ちたら timeout を待たずに失敗する", func(t *testing.T) {
		t.Parallel()

		client := &bootingClient{readyAfter: 1 << 30}
		repo := &startingHostRepo{client: client}
		repo.status.Store(int32(entity.HeadlessHostStatus_CRASHED))

		hhuc := newHostUsecaseUnderTest(repo)
		hhuc.hostReadyTimeout = time.Hour

		err := hhuc.waitUntilReady(t.Context(), "h-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exited before it became ready")
		assert.Zero(t, client.probes.Load())
	})

	t.Run("ready にならないまま上限に達したら失敗する", func(t *testing.T) {
		t.Parallel()

		repo := &startingHostRepo{client: &bootingClient{readyAfter: 1 << 30}}
		repo.status.Store(int32(entity.HeadlessHostStatus_RUNNING))

		hhuc := newHostUsecaseUnderTest(repo)
		hhuc.hostReadyTimeout = 20 * time.Millisecond

		err := hhuc.waitUntilReady(t.Context(), "h-1")
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
