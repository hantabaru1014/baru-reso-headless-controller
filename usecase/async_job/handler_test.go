package async_job

import (
	"context"
	"testing"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// stubHostOperator は HeadlessHostRestart の戻り値を固定する stub.
type stubHostOperator struct {
	HostOperator

	restartErr    error
	restartCalled bool
}

func (s *stubHostOperator) HeadlessHostRestart(_ context.Context, _ string, _ *string, _ bool, _ int) error {
	s.restartCalled = true

	return s.restartErr
}

// stubImageBuildOperator は RunBuild の結果を固定する stub.
type stubImageBuildOperator struct {
	tag string
}

func (s *stubImageBuildOperator) RunBuild(_ context.Context, _ string, _ entity.ResoniteVersionBranch) (string, error) {
	return s.tag, nil
}

func newDispatcherUnderTest(repo port.AsyncJobRepository, host HostOperator, image ImageBuildOperator) *Dispatcher {
	uc := NewUsecase(repo, &stubPermChecker{allow: true})

	return NewDispatcher(host, nil, nil, image, uc)
}

// 停止済みホストを latestRelease で再起動したとき、その版がまだビルドされて
// いなければ job を失敗させず BUILD_IMAGE を chain する (START_HOST と同じ扱い).
func TestDispatcher_RestartHost_ChainsBuildWhenNotBuilt(t *testing.T) {
	t.Parallel()

	repo := &stubAsyncJobRepo{}
	host := &stubHostOperator{
		// usecase 層は go-errors で wrap して返すため、テストでも wrap しておく.
		restartErr: errors.Wrap(&usecase.NotBuiltError{
			ManifestID: "m-new",
			Branch:     entity.ResoniteVersionBranch_Headless,
		}, 0),
	}

	payload, err := protojson.Marshal(&hdlctrlv1.RestartHeadlessHostRequest{
		HostId:           "h-1",
		WithUpdate:       true,
		WithWorldRestart: true,
	})
	require.NoError(t, err)

	createdBy := "U-caller"

	result, msg, err := newDispatcherUnderTest(repo, host, nil).Dispatch(t.Context(), &entity.AsyncJob{
		ID:        "job-restart",
		JobType:   entity.AsyncJobType_RESTART_HOST,
		Payload:   payload,
		CreatedBy: &createdBy,
	})
	require.NoError(t, err)
	assert.Equal(t, "h-1", result.HostID)
	assert.Contains(t, msg, "ビルドキューに追加")

	require.Len(t, repo.created, 1)
	assert.Equal(t, entity.AsyncJobType_BUILD_IMAGE, repo.created[0].JobType)
	// job 履歴でホストに紐付けられるよう、再起動 chain の build job は host_id を持つ.
	require.NotNil(t, repo.created[0].HostID)
	assert.Equal(t, "h-1", *repo.created[0].HostID)

	build := &hdlctrlv1.BuildResoniteImageRequest{}
	require.NoError(t, protojson.Unmarshal(repo.created[0].Payload, build))
	assert.Equal(t, "m-new", build.GetManifestId())
	assert.Equal(t, string(entity.ResoniteVersionBranch_Headless), build.GetBranch())

	// chain 先には元の再起動リクエストがそのまま載る (world 復元指定等を失わない).
	require.NotNil(t, build.GetThenRestartHost())
	assert.Equal(t, "h-1", build.GetThenRestartHost().GetHostId())
	assert.True(t, build.GetThenRestartHost().GetWithWorldRestart())
	// 新規ホスト起動用の chain は使わない.
	assert.Nil(t, build.GetThenStartHost())
}

func TestDispatcher_BuildImage_ChainsRestartWithBuiltTag(t *testing.T) {
	t.Parallel()

	repo := &stubAsyncJobRepo{}

	payload, err := protojson.Marshal(&hdlctrlv1.BuildResoniteImageRequest{
		ManifestId: "m-new",
		Branch:     string(entity.ResoniteVersionBranch_Headless),
		FollowUp: &hdlctrlv1.BuildResoniteImageRequest_ThenRestartHost{
			ThenRestartHost: &hdlctrlv1.RestartHeadlessHostRequest{
				HostId:           "h-1",
				WithUpdate:       true,
				WithWorldRestart: true,
			},
		},
	})
	require.NoError(t, err)

	createdBy := "U-caller"

	_, msg, err := newDispatcherUnderTest(repo, nil, &stubImageBuildOperator{tag: "2026.8.1-headless"}).
		Dispatch(t.Context(), &entity.AsyncJob{
			ID:        "job-build",
			JobType:   entity.AsyncJobType_BUILD_IMAGE,
			Payload:   payload,
			CreatedBy: &createdBy,
		})
	require.NoError(t, err)
	assert.Contains(t, msg, "host_restart_job=")

	require.Len(t, repo.created, 1)
	assert.Equal(t, entity.AsyncJobType_RESTART_HOST, repo.created[0].JobType)
	require.NotNil(t, repo.created[0].HostID)
	assert.Equal(t, "h-1", *repo.created[0].HostID)

	restart := &hdlctrlv1.RestartHeadlessHostRequest{}
	require.NoError(t, protojson.Unmarshal(repo.created[0].Payload, restart))
	// ビルドしたタグで起動する. with_update のままだと latestRelease を再解決して
	// しまい、ビルド直後に更に新しい版が出ていると無限に追いかけることになる.
	assert.Equal(t, "2026.8.1-headless", restart.GetWithImageTag())
	assert.False(t, restart.GetWithUpdate())
	assert.True(t, restart.GetWithWorldRestart())
}

// 未 built 以外の失敗は今まで通り job を失敗させる (build を無駄に走らせない).
func TestDispatcher_RestartHost_DoesNotChainOnOtherErrors(t *testing.T) {
	t.Parallel()

	repo := &stubAsyncJobRepo{}
	host := &stubHostOperator{restartErr: errors.New("container start failed")}

	payload, err := protojson.Marshal(&hdlctrlv1.RestartHeadlessHostRequest{HostId: "h-1", WithUpdate: true})
	require.NoError(t, err)

	createdBy := "U-caller"

	_, _, err = newDispatcherUnderTest(repo, host, nil).Dispatch(t.Context(), &entity.AsyncJob{
		ID:        "job-restart",
		JobType:   entity.AsyncJobType_RESTART_HOST,
		Payload:   payload,
		CreatedBy: &createdBy,
	})
	require.Error(t, err)
	assert.True(t, host.restartCalled)
	assert.Empty(t, repo.created)
}
