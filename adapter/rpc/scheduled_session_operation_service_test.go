package rpc

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// setupScheduledOpTarget は scheduled-op テストのために、権限解決対象として
// 必要な host + session を作る. session.id は明示的に指定する (CreateTestSession は
// ID をランダム生成するため使わない).
func setupScheduledOpTarget(t *testing.T, queries *db.Queries, sessionID string) {
	t.Helper()
	testutil.CreateTestHeadlessAccount(t, queries, "U-sched-acc", "sched@example.test", "p")
	h := testutil.CreateTestHeadlessHost(t, queries, "U-sched-acc", "sched-host", entity.HeadlessHostStatus_RUNNING)
	_, err := queries.UpsertSession(t.Context(), db.UpsertSessionParams{
		ID:                             sessionID,
		Name:                           sessionID,
		Status:                         int32(entity.SessionStatus_RUNNING),
		StartedAt:                      pgtype.Timestamptz{Valid: true, Time: time.Now()},
		CreatedBy:                      pgtype.Text{Valid: false},
		GroupID:                        entity.MigratedPrePermissionGroupID,
		EndedAt:                        pgtype.Timestamptz{Valid: false},
		HostID:                         h.ID,
		StartupParameters:              []byte(`{"maxUsers": 8}`),
		StartupParametersSchemaVersion: 1,
		AutoUpgrade:                    false,
		Memo:                           pgtype.Text{Valid: false},
	})
	require.NoError(t, err)
}

func TestControllerService_ScheduledSessionOperations(t *testing.T) {
	t.Run("成功: STOP_SESSION を Time trigger で予約 → 一覧 → キャンセル", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// scheduled-op の権限解決のために対象 session を実体として作成しておく.
		setupScheduledOpTarget(t, setup.queries, "S-target")

		// Create
		scheduledAt := time.Now().Add(time.Hour).UTC()
		createReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateScheduledSessionOperationRequest{
			Operation: &hdlctrlv1.ScheduledOperation{
				Operation: &hdlctrlv1.ScheduledOperation_StopSession{
					StopSession: &hdlctrlv1.StopSessionRequest{SessionId: "S-target"},
				},
			},
			Trigger: &hdlctrlv1.ScheduledTrigger{
				Trigger: &hdlctrlv1.ScheduledTrigger_Time{
					Time: &hdlctrlv1.TimeTrigger{ScheduledAt: timestamppb.New(scheduledAt)},
				},
			},
		})
		createRes, err := client.CreateScheduledSessionOperation(t.Context(), createReq)
		require.NoError(t, err)
		require.NotNil(t, createRes.Msg.GetScheduledOperation())

		opID := createRes.Msg.GetScheduledOperation().GetId()
		assert.NotEmpty(t, opID)
		assert.Equal(t, hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_PENDING, createRes.Msg.GetScheduledOperation().GetStatus())
		assert.Equal(t, "S-target", createRes.Msg.GetScheduledOperation().GetSessionId())
		assert.Equal(t, scheduledAt.Unix(), createRes.Msg.GetScheduledOperation().GetTrigger().GetTime().GetScheduledAt().AsTime().Unix())

		// List - フィルタなし
		listReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListScheduledSessionOperationsRequest{})
		listRes, err := client.ListScheduledSessionOperations(t.Context(), listReq)
		require.NoError(t, err)
		assert.Len(t, listRes.Msg.GetScheduledOperations(), 1)

		// List - session_id フィルタ
		targetSID := "S-target"
		listFilteredReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListScheduledSessionOperationsRequest{
			SessionId: &targetSID,
		})
		listFilteredRes, err := client.ListScheduledSessionOperations(t.Context(), listFilteredReq)
		require.NoError(t, err)
		assert.Len(t, listFilteredRes.Msg.GetScheduledOperations(), 1)

		// List - 別 session_id (空)
		other := "S-other"
		listOtherReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListScheduledSessionOperationsRequest{
			SessionId: &other,
		})
		listOtherRes, err := client.ListScheduledSessionOperations(t.Context(), listOtherReq)
		require.NoError(t, err)
		assert.Empty(t, listOtherRes.Msg.GetScheduledOperations())

		// Cancel
		cancelReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CancelScheduledSessionOperationRequest{Id: opID})
		_, err = client.CancelScheduledSessionOperation(t.Context(), cancelReq)
		require.NoError(t, err)

		// Cancel 2 回目 → FailedPrecondition
		_, err = client.CancelScheduledSessionOperation(t.Context(), cancelReq)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeFailedPrecondition, connectErr.Code())
	})

	t.Run("成功: STOP_SESSION を SessionUserCount trigger で予約", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		setupScheduledOpTarget(t, setup.queries, "S-cond")

		createReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateScheduledSessionOperationRequest{
			Operation: &hdlctrlv1.ScheduledOperation{
				Operation: &hdlctrlv1.ScheduledOperation_StopSession{
					StopSession: &hdlctrlv1.StopSessionRequest{SessionId: "S-cond"},
				},
			},
			Trigger: &hdlctrlv1.ScheduledTrigger{
				Trigger: &hdlctrlv1.ScheduledTrigger_SessionUserCount{
					SessionUserCount: &hdlctrlv1.SessionUserCountTrigger{
						SessionId:  "S-cond",
						Comparator: hdlctrlv1.SessionUserCountTrigger_COMPARATOR_LESS_OR_EQUAL,
						Threshold:  0,
					},
				},
			},
		})
		createRes, err := client.CreateScheduledSessionOperation(t.Context(), createReq)
		require.NoError(t, err)
		require.NotNil(t, createRes.Msg.GetScheduledOperation())

		got := createRes.Msg.GetScheduledOperation()
		assert.Equal(t, "S-cond", got.GetSessionId())
		assert.Equal(t, hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_PENDING, got.GetStatus())

		condTrig := got.GetTrigger().GetSessionUserCount()
		require.NotNil(t, condTrig)
		assert.Equal(t, "S-cond", condTrig.GetSessionId())
		assert.Equal(t, hdlctrlv1.SessionUserCountTrigger_COMPARATOR_LESS_OR_EQUAL, condTrig.GetComparator())
		assert.Equal(t, int32(0), condTrig.GetThreshold())
	})

	t.Run("失敗: SessionUserCount trigger の comparator 未指定で InvalidArgument", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateScheduledSessionOperationRequest{
			Operation: &hdlctrlv1.ScheduledOperation{
				Operation: &hdlctrlv1.ScheduledOperation_StopSession{
					StopSession: &hdlctrlv1.StopSessionRequest{SessionId: "S-x"},
				},
			},
			Trigger: &hdlctrlv1.ScheduledTrigger{
				Trigger: &hdlctrlv1.ScheduledTrigger_SessionUserCount{
					SessionUserCount: &hdlctrlv1.SessionUserCountTrigger{
						SessionId: "S-x",
						Threshold: 0,
					},
				},
			},
		})
		_, err := client.CreateScheduledSessionOperation(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("失敗: operation 未指定で InvalidArgument", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateScheduledSessionOperationRequest{
			Trigger: &hdlctrlv1.ScheduledTrigger{
				Trigger: &hdlctrlv1.ScheduledTrigger_Time{
					Time: &hdlctrlv1.TimeTrigger{ScheduledAt: timestamppb.New(time.Now().Add(time.Hour))},
				},
			},
		})
		_, err := client.CreateScheduledSessionOperation(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}
