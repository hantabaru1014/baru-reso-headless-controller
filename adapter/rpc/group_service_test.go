package rpc

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/dchest/uniuri"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type groupServiceTestSetup struct {
	*userServiceTestSetup
	userClient  hdlctrlv1connect.UserServiceClient
	groupClient hdlctrlv1connect.GroupServiceClient
}

func setupGroupServiceTest(t *testing.T) *groupServiceTestSetup {
	t.Helper()

	us := setupUserServiceTest(t)
	t.Cleanup(us.Cleanup)

	us.mockSkyfrost.EXPECT().
		FetchUserInfo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, id string) (*skyfrost.UserInfo, error) {
			return &skyfrost.UserInfo{ID: id, UserName: id}, nil
		}).
		AnyTimes()

	permUC := newPermissionUsecaseForTest(us.queries)
	guc := newGroupUsecaseForTest(us.queries, permUC)
	service := NewGroupService(
		guc,
		permUC,
		adapter.NewGroupRepository(us.queries),
		adapter.NewRoleRepository(us.queries),
		nil, nil, nil,
	)

	server := testutil.SetupAuthenticatedHTTPServer(t, service)
	t.Cleanup(server.Close)

	return &groupServiceTestSetup{
		userServiceTestSetup: us,
		userClient:           setupUserServiceClient(t, us.service),
		groupClient:          hdlctrlv1connect.NewGroupServiceClient(server.Client(), server.URL),
	}
}

const invitedResoniteID = "U-invited"

// createInvitation は invitedResoniteID 宛の招待を発行し (招待 ID, 平文トークン) を返す.
func (s *groupServiceTestSetup) createInvitation(t *testing.T) (string, string) {
	t.Helper()

	res, err := s.userClient.CreateRegistrationToken(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateRegistrationTokenRequest{
		ResoniteId: invitedResoniteID,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, res.Msg.GetInvitationId())

	return res.Msg.GetInvitationId(), res.Msg.GetToken()
}

func TestGroupService_InvitedGroupMember(t *testing.T) {
	t.Run("成功: 招待中ユーザーをグループに追加し、登録時にメンバーになる", func(t *testing.T) {
		s := setupGroupServiceTest(t)

		groupID := "g-" + uniuri.New()
		testutil.CreateTestGroup(t, s.queries, groupID, "")
		invitationID, token := s.createInvitation(t)

		addRes, err := s.groupClient.AddInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AddInvitedGroupMemberRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_User,
		}))
		require.NoError(t, err)
		assert.Equal(t, invitedResoniteID, addRes.Msg.GetMember().GetResoniteId())
		assert.Equal(t, "test@example.test", addRes.Msg.GetMember().GetAddedBy())
		assert.NotNil(t, addRes.Msg.GetMember().GetExpiresAt())

		updateRes, err := s.groupClient.UpdateInvitedGroupMemberRole(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateInvitedGroupMemberRoleRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_Admin,
		}))
		require.NoError(t, err)
		assert.Equal(t, entity.SeedRoleID_Admin, updateRes.Msg.GetMember().GetRoleId())

		listRes, err := s.groupClient.ListGroupMembers(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListGroupMembersRequest{GroupId: groupID}))
		require.NoError(t, err)
		assert.Empty(t, listRes.Msg.GetMembers())
		require.Len(t, listRes.Msg.GetInvitedMembers(), 1)
		assert.Equal(t, invitationID, listRes.Msg.GetInvitedMembers()[0].GetInvitationId())

		// 再発行しても参加予定は引き継がれ、新しいトークンで登録できる.
		reissueRes, err := s.userClient.ReissueInvitation(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ReissueInvitationRequest{
			InvitationId: invitationID,
		}))
		require.NoError(t, err)
		assert.NotEqual(t, token, reissueRes.Msg.GetToken())

		_, err = s.userClient.RegisterWithToken(t.Context(), connect.NewRequest(&hdlctrlv1.RegisterWithTokenRequest{
			Token:    token,
			UserId:   "invited-user",
			Password: "password123",
		}))
		require.Error(t, err, "旧トークンは再発行で無効になる")

		_, err = s.userClient.RegisterWithToken(t.Context(), connect.NewRequest(&hdlctrlv1.RegisterWithTokenRequest{
			Token:    reissueRes.Msg.GetToken(),
			UserId:   "invited-user",
			Password: "password123",
		}))
		require.NoError(t, err)

		listRes, err = s.groupClient.ListGroupMembers(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListGroupMembersRequest{GroupId: groupID}))
		require.NoError(t, err)
		assert.Empty(t, listRes.Msg.GetInvitedMembers())
		require.Len(t, listRes.Msg.GetMembers(), 1)
		member := listRes.Msg.GetMembers()[0]
		assert.Equal(t, "invited-user", member.GetUserId())
		assert.Equal(t, entity.SeedRoleID_Admin, member.GetRoleId())
		assert.Equal(t, "test@example.test", member.GetAddedBy())

		// 登録済みの招待には参加予定を追加できない.
		_, err = s.groupClient.AddInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AddInvitedGroupMemberRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_User,
		}))
		assertConnectCode(t, err, connect.CodeNotFound)
	})

	t.Run("成功: 参加予定の削除と招待の取消", func(t *testing.T) {
		s := setupGroupServiceTest(t)

		groupID := "g-" + uniuri.New()
		testutil.CreateTestGroup(t, s.queries, groupID, "")
		invitationID, _ := s.createInvitation(t)

		add := func() {
			_, err := s.groupClient.AddInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AddInvitedGroupMemberRequest{
				GroupId:      groupID,
				InvitationId: invitationID,
				RoleId:       entity.SeedRoleID_User,
			}))
			require.NoError(t, err)
		}
		countInvited := func() int {
			res, err := s.groupClient.ListGroupMembers(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListGroupMembersRequest{GroupId: groupID}))
			require.NoError(t, err)

			return len(res.Msg.GetInvitedMembers())
		}

		add()

		// 同じ招待の重複追加は AlreadyExists.
		_, err := s.groupClient.AddInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AddInvitedGroupMemberRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_User,
		}))
		assertConnectCode(t, err, connect.CodeAlreadyExists)

		// 別グループを指定しても他グループの参加予定は操作できない.
		otherGroupID := "g-" + uniuri.New()
		testutil.CreateTestGroup(t, s.queries, otherGroupID, "")

		_, err = s.groupClient.UpdateInvitedGroupMemberRole(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateInvitedGroupMemberRoleRequest{
			GroupId:      otherGroupID,
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_Admin,
		}))
		assertConnectCode(t, err, connect.CodeNotFound)

		_, err = s.groupClient.RemoveInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RemoveInvitedGroupMemberRequest{
			GroupId:      otherGroupID,
			InvitationId: invitationID,
		}))
		assertConnectCode(t, err, connect.CodeNotFound)
		assert.Equal(t, 1, countInvited())

		_, err = s.groupClient.RemoveInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RemoveInvitedGroupMemberRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
		}))
		require.NoError(t, err)
		assert.Equal(t, 0, countInvited())

		_, err = s.groupClient.RemoveInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RemoveInvitedGroupMemberRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
		}))
		assertConnectCode(t, err, connect.CodeNotFound)

		// 招待を取り消すと参加予定も消える.
		add()

		_, err = s.userClient.RevokeInvitation(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RevokeInvitationRequest{
			InvitationId: invitationID,
		}))
		require.NoError(t, err)
		assert.Equal(t, 0, countInvited())

		listRes, err := s.userClient.ListInvitations(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListInvitationsRequest{}))
		require.NoError(t, err)
		assert.Empty(t, listRes.Msg.GetInvitations())
	})

	t.Run("失敗: personal グループは対象外 → FailedPrecondition", func(t *testing.T) {
		s := setupGroupServiceTest(t)

		invitationID, _ := s.createInvitation(t)

		_, err := s.groupClient.AddInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AddInvitedGroupMemberRequest{
			GroupId:      "test@example.test-personal",
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_User,
		}))
		assertConnectCode(t, err, connect.CodeFailedPrecondition)

		_, err = s.groupClient.UpdateInvitedGroupMemberRole(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateInvitedGroupMemberRoleRequest{
			GroupId:      "test@example.test-personal",
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_User,
		}))
		assertConnectCode(t, err, connect.CodeFailedPrecondition)
	})

	t.Run("失敗: 存在しない招待 → NotFound", func(t *testing.T) {
		s := setupGroupServiceTest(t)

		groupID := "g-" + uniuri.New()
		testutil.CreateTestGroup(t, s.queries, groupID, "")

		_, err := s.groupClient.AddInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AddInvitedGroupMemberRequest{
			GroupId:      groupID,
			InvitationId: "no-such-invitation",
			RoleId:       entity.SeedRoleID_User,
		}))
		assertConnectCode(t, err, connect.CodeNotFound)
	})

	t.Run("失敗: group:members.manage 権限なし → PermissionDenied", func(t *testing.T) {
		s := setupGroupServiceTest(t)

		groupID := "g-" + uniuri.New()
		testutil.SetupUserWithExactPermissions(t, s.queries, "viewer@example.test", groupID, []string{entity.PermKey_GroupEdit})
		invitationID, _ := s.createInvitation(t)

		_, err := s.groupClient.AddInvitedGroupMember(t.Context(), testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.AddInvitedGroupMemberRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_User,
		}, "viewer@example.test", "U-viewer", ""))
		assertConnectCode(t, err, connect.CodePermissionDenied)
	})

	t.Run("失敗: 自分が持たない権限を含むロールでは追加・変更できない → PermissionDenied", func(t *testing.T) {
		s := setupGroupServiceTest(t)

		groupID := "g-" + uniuri.New()
		testutil.SetupUserWithExactPermissions(t, s.queries, "manager@example.test", groupID, []string{entity.PermKey_GroupMembersManage})
		invitationID, _ := s.createInvitation(t)

		_, err := s.groupClient.AddInvitedGroupMember(t.Context(), testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.AddInvitedGroupMemberRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_Admin,
		}, "manager@example.test", "U-manager", ""))
		assertConnectCode(t, err, connect.CodePermissionDenied)

		// system-admin が追加した参加予定のロールを引き上げることもできない.
		_, err = s.groupClient.AddInvitedGroupMember(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AddInvitedGroupMemberRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_User,
		}))
		require.NoError(t, err)

		_, err = s.groupClient.UpdateInvitedGroupMemberRole(t.Context(), testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.UpdateInvitedGroupMemberRoleRequest{
			GroupId:      groupID,
			InvitationId: invitationID,
			RoleId:       entity.SeedRoleID_Admin,
		}, "manager@example.test", "U-manager", ""))
		assertConnectCode(t, err, connect.CodePermissionDenied)
	})
}

func assertConnectCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	require.Error(t, err)

	connectErr := &connect.Error{}
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, code, connectErr.Code(), connectErr.Message())
}
