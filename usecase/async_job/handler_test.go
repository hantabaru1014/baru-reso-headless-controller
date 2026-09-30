package async_job

import (
	"context"
	"testing"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// stubHostOperator は HeadlessHostRestart / HeadlessHostEnsureRunning の戻り値を固定する stub.
type stubHostOperator struct {
	HostOperator

	restartErr    error
	restartCalled bool

	ensureStarted bool
	ensureErr     error
	// calls は session 側の stub と共有する呼び出し順の記録.
	calls *[]string
}

func (s *stubHostOperator) HeadlessHostRestart(_ context.Context, _ string, _ *string, _ bool, _ int) error {
	s.restartCalled = true

	return s.restartErr
}

func (s *stubHostOperator) HeadlessHostEnsureRunning(_ context.Context, id string) (bool, error) {
	*s.calls = append(*s.calls, "ensure:"+id)

	return s.ensureStarted, s.ensureErr
}

// stubSessionOperator は StartSession の呼び出しを記録する stub.
type stubSessionOperator struct {
	SessionOperator

	calls *[]string
}

func (s *stubSessionOperator) StartSession(_ context.Context, hostID string, _ string, _ *string, _ *headlessv1.WorldStartupParameters, _ *string) (*entity.Session, error) {
	*s.calls = append(*s.calls, "start:"+hostID)

	return &entity.Session{ID: "S-new"}, nil
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

func startSessionJob(t *testing.T, hostID string) *entity.AsyncJob {
	t.Helper()

	name := "MyWorld"

	payload, err := protojson.Marshal(&hdlctrlv1.StartWorldRequest{
		HostId:     hostID,
		Parameters: &headlessv1.WorldStartupParameters{Name: &name},
	})
	require.NoError(t, err)

	createdBy := "U-caller"

	return &entity.AsyncJob{
		ID:        "job-start-session",
		JobType:   entity.AsyncJobType_START_SESSION,
		Payload:   payload,
		CreatedBy: &createdBy,
	}
}

// 停止中のホストが指定されたら、ホストを起動してからセッションを開始する.
func TestDispatcher_StartSession_EnsuresHostRunningFirst(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		hostStarted bool
		wantMsg     string
	}{
		{name: "停止中ホストを起動した", hostStarted: true, wantMsg: `ホストを起動し、セッション "MyWorld" を開始しました`},
		{name: "起動済みホスト", hostStarted: false, wantMsg: `セッション "MyWorld" を開始しました`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var calls []string

			host := &stubHostOperator{ensureStarted: tc.hostStarted, calls: &calls}
			session := &stubSessionOperator{calls: &calls}
			d := NewDispatcher(host, session, nil, nil, NewUsecase(&stubAsyncJobRepo{}, &stubPermChecker{allow: true}))

			result, msg, err := d.Dispatch(t.Context(), startSessionJob(t, "h-1"))
			require.NoError(t, err)
			assert.Equal(t, []string{"ensure:h-1", "start:h-1"}, calls)
			assert.Equal(t, "S-new", result.SessionID)
			assert.Equal(t, "h-1", result.HostID)
			assert.Equal(t, tc.wantMsg, msg)
		})
	}
}

// 停止中ホストの起動に使うイメージが未 built なら、セッションは開始せずに BUILD_IMAGE を
// chain し、ビルド後に同じ StartWorld が再実行されるようにする.
func TestDispatcher_StartSession_ChainsBuildWhenHostImageNotBuilt(t *testing.T) {
	t.Parallel()

	var calls []string

	repo := &stubAsyncJobRepo{}
	host := &stubHostOperator{
		ensureErr: errors.Wrap(&usecase.NotBuiltError{
			ManifestID: "m-new",
			Branch:     entity.ResoniteVersionBranch_Headless,
		}, 0),
		calls: &calls,
	}
	session := &stubSessionOperator{calls: &calls}
	d := NewDispatcher(host, session, nil, nil, NewUsecase(repo, &stubPermChecker{allow: true}))

	result, msg, err := d.Dispatch(t.Context(), startSessionJob(t, "h-1"))
	require.NoError(t, err)
	assert.Equal(t, []string{"ensure:h-1"}, calls)
	assert.Equal(t, "h-1", result.HostID)
	assert.Empty(t, result.SessionID)
	assert.Contains(t, msg, "ビルドキューに追加")

	require.Len(t, repo.created, 1)
	assert.Equal(t, entity.AsyncJobType_BUILD_IMAGE, repo.created[0].JobType)
	require.NotNil(t, repo.created[0].HostID)
	assert.Equal(t, "h-1", *repo.created[0].HostID)

	build := &hdlctrlv1.BuildResoniteImageRequest{}
	require.NoError(t, protojson.Unmarshal(repo.created[0].Payload, build))
	assert.Equal(t, "m-new", build.GetManifestId())
	require.NotNil(t, build.GetThenStartWorld())
	assert.Equal(t, "h-1", build.GetThenStartWorld().GetHostId())
	assert.Equal(t, "MyWorld", build.GetThenStartWorld().GetParameters().GetName())
}

// ホストを起動できなかった場合はセッションを開始しない.
func TestDispatcher_StartSession_FailsWhenHostCannotStart(t *testing.T) {
	t.Parallel()

	var calls []string

	repo := &stubAsyncJobRepo{}
	host := &stubHostOperator{ensureErr: errors.New("host exited before it became ready"), calls: &calls}
	session := &stubSessionOperator{calls: &calls}
	d := NewDispatcher(host, session, nil, nil, NewUsecase(repo, &stubPermChecker{allow: true}))

	_, _, err := d.Dispatch(t.Context(), startSessionJob(t, "h-1"))
	require.Error(t, err)
	assert.Equal(t, []string{"ensure:h-1"}, calls)
	assert.Empty(t, repo.created)
}

func TestDispatcher_BuildImage_ChainsStartSession(t *testing.T) {
	t.Parallel()

	repo := &stubAsyncJobRepo{}

	payload, err := protojson.Marshal(&hdlctrlv1.BuildResoniteImageRequest{
		ManifestId: "m-new",
		Branch:     string(entity.ResoniteVersionBranch_Headless),
		FollowUp: &hdlctrlv1.BuildResoniteImageRequest_ThenStartWorld{
			ThenStartWorld: &hdlctrlv1.StartWorldRequest{HostId: "h-1", Memo: "memo"},
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
	assert.Contains(t, msg, "session_start_job=")

	require.Len(t, repo.created, 1)
	assert.Equal(t, entity.AsyncJobType_START_SESSION, repo.created[0].JobType)
	require.NotNil(t, repo.created[0].HostID)
	assert.Equal(t, "h-1", *repo.created[0].HostID)
	require.NotNil(t, repo.created[0].CreatedBy)
	assert.Equal(t, createdBy, *repo.created[0].CreatedBy)

	start := &hdlctrlv1.StartWorldRequest{}
	require.NoError(t, protojson.Unmarshal(repo.created[0].Payload, start))
	assert.Equal(t, "h-1", start.GetHostId())
	assert.Equal(t, "memo", start.GetMemo())
}
