package async_job

import (
	"context"
	"testing"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAsyncJobRepo は List に渡された filter を記録するだけの stub.
// 認可の結果 CreatedBy がどう決まったかを検証するために使う.
type stubAsyncJobRepo struct {
	port.AsyncJobRepository

	called    bool
	gotFilter port.AsyncJobListFilter

	// created は Create された job の記録 (chain の検証に使う).
	created []port.AsyncJobCreateParams
}

func (s *stubAsyncJobRepo) Create(_ context.Context, params port.AsyncJobCreateParams) (*entity.AsyncJob, error) {
	s.created = append(s.created, params)

	return &entity.AsyncJob{ID: "job-chained", JobType: params.JobType}, nil
}

func (s *stubAsyncJobRepo) List(_ context.Context, filter port.AsyncJobListFilter) (*port.AsyncJobListResult, error) {
	s.called = true
	s.gotFilter = filter

	return &port.AsyncJobListResult{}, nil
}

// stubPermChecker は system 権限の有無を固定で返す.
type stubPermChecker struct {
	allow bool
	// gotPermKey は要求された permission key. 権限チェックが走ったかの判定も兼ねる.
	gotPermKey string
}

func (s *stubPermChecker) RequireSystemPermission(_ context.Context, permKey string) error {
	s.gotPermKey = permKey

	if !s.allow {
		return domain.ErrPermissionDenied
	}

	return nil
}

const callerID = "caller@example.test"

// actAsCaller は認可の caller を埋め込んだ ctx を返す.
func actAsCaller(ctx context.Context) context.Context {
	return auth.WithActAsUser(ctx, callerID)
}

func TestUsecase_List_Authorization(t *testing.T) {
	t.Run("成功: 既定では caller 自身の job に強制的に絞られる", func(t *testing.T) {
		repo := &stubAsyncJobRepo{}
		perm := &stubPermChecker{allow: false}
		uc := NewUsecase(repo, perm)

		status := entity.AsyncJobStatus_FAILED

		_, err := uc.List(actAsCaller(t.Context()), ListFilter{Status: &status, PageSize: 20})
		require.NoError(t, err)

		require.NotNil(t, repo.gotFilter.CreatedBy)
		assert.Equal(t, callerID, *repo.gotFilter.CreatedBy)
		// 自分の job だけなら system 権限チェックは走らない.
		assert.Empty(t, perm.gotPermKey)
		// 認可に関係ない filter はそのまま repo に渡る.
		require.NotNil(t, repo.gotFilter.Status)
		assert.Equal(t, entity.AsyncJobStatus_FAILED, *repo.gotFilter.Status)
		assert.Equal(t, int32(20), repo.gotFilter.PageSize)
	})

	t.Run("成功: system 権限があれば include_all_users で created_by 絞り込みなし", func(t *testing.T) {
		repo := &stubAsyncJobRepo{}
		perm := &stubPermChecker{allow: true}
		uc := NewUsecase(repo, perm)

		_, err := uc.List(actAsCaller(t.Context()), ListFilter{IncludeAllUsers: true})
		require.NoError(t, err)

		assert.Nil(t, repo.gotFilter.CreatedBy)
		assert.Equal(t, entity.PermKey_SystemGroupManage, perm.gotPermKey)
	})

	t.Run("失敗: system 権限なしの include_all_users は PermissionDenied", func(t *testing.T) {
		repo := &stubAsyncJobRepo{}
		perm := &stubPermChecker{allow: false}
		uc := NewUsecase(repo, perm)

		_, err := uc.List(actAsCaller(t.Context()), ListFilter{IncludeAllUsers: true})
		require.ErrorIs(t, err, domain.ErrPermissionDenied)

		// 拒否されたら repo には到達しない.
		assert.False(t, repo.called)
	})

	t.Run("失敗: 未認証なら Unauthenticated", func(t *testing.T) {
		repo := &stubAsyncJobRepo{}
		perm := &stubPermChecker{allow: true}
		uc := NewUsecase(repo, perm)

		_, err := uc.List(t.Context(), ListFilter{})
		require.ErrorIs(t, err, domain.ErrUnauthenticated)
		assert.False(t, repo.called)
	})
}
