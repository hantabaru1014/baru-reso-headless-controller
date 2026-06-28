// group_service.go は GroupService の connect RPC handler.
// 仕様: docs/permissions.md 9.
package rpc

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/logging"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ hdlctrlv1connect.GroupServiceHandler = (*GroupService)(nil)

type GroupService struct {
	guc       *usecase.GroupUsecase
	permUC    *usecase.PermissionUsecase
	groupRepo port.GroupRepository
	roleRepo  port.RoleRepository

	// permission interceptor 依存
	hostRepo    port.HeadlessHostRepository
	sessionRepo port.SessionRepository
	hauc        *usecase.HeadlessAccountUsecase
}

func NewGroupService(
	guc *usecase.GroupUsecase,
	permUC *usecase.PermissionUsecase,
	groupRepo port.GroupRepository,
	roleRepo port.RoleRepository,
	hostRepo port.HeadlessHostRepository,
	sessionRepo port.SessionRepository,
	hauc *usecase.HeadlessAccountUsecase,
) *GroupService {
	return &GroupService{
		guc:         guc,
		permUC:      permUC,
		groupRepo:   groupRepo,
		roleRepo:    roleRepo,
		hostRepo:    hostRepo,
		sessionRepo: sessionRepo,
		hauc:        hauc,
	}
}

func (s *GroupService) NewHandler() (string, http.Handler) {
	interceptors := connect.WithInterceptors(
		logging.NewErrorLogInterceptor(),
		auth.NewAuthInterceptor(),
		NewPermissionInterceptor(s.permUC, PermissionDeps{
			HostRepo:    s.hostRepo,
			SessionRepo: s.sessionRepo,
			AccountUC:   s.hauc,
			GroupRepo:   s.groupRepo,
			RoleRepo:    s.roleRepo,
		}),
	)

	return hdlctrlv1connect.NewGroupServiceHandler(s, interceptors)
}

// CreateGroup creates a new normal group with the caller as admin.
// 権限: system:group.manage. 任意ユーザーがグループを乱造できないようにする.
var _ = registerRPCPermission(
	hdlctrlv1connect.GroupServiceCreateGroupProcedure,
	requireSystemPerm(entity.PermKey_SystemGroupManage),
)

func (s *GroupService) CreateGroup(ctx context.Context, req *connect.Request[hdlctrlv1.CreateGroupRequest]) (*connect.Response[hdlctrlv1.CreateGroupResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	g, err := s.guc.CreateGroup(ctx, req.Msg.GetName(), claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.CreateGroupResponse{Group: groupToProto(g)}), nil
}

// GetGroup: 所属しているか system:group.list を持つなら閲覧可.
var _ = registerRPCPermission(
	hdlctrlv1connect.GroupServiceGetGroupProcedure,
	checkGroupPermission(entity.PermKey_GroupEdit, groupIDFromGet, true),
)

func (s *GroupService) GetGroup(ctx context.Context, req *connect.Request[hdlctrlv1.GetGroupRequest]) (*connect.Response[hdlctrlv1.GetGroupResponse], error) {
	g, err := s.guc.GetGroup(ctx, req.Msg.GetGroupId())
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.GetGroupResponse{Group: groupToProto(g)}), nil
}

// ListGroups: handler 側で claims から自身の所属グループに絞る (interceptor は通過のみ).
var _ = registerRPCPermission(
	hdlctrlv1connect.GroupServiceListGroupsProcedure,
	requireAuthOnly,
)

func (s *GroupService) ListGroups(ctx context.Context, _ *connect.Request[hdlctrlv1.ListGroupsRequest]) (*connect.Response[hdlctrlv1.ListGroupsResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	groups, err := s.guc.ListGroupsForUser(ctx, claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	protoGroups := make([]*hdlctrlv1.Group, 0, len(groups))
	for _, g := range groups {
		protoGroups = append(protoGroups, groupToProto(g))
	}

	return connect.NewResponse(&hdlctrlv1.ListGroupsResponse{Groups: protoGroups}), nil
}

// UpdateGroup: group_id に対して group:edit.
var _ = registerRPCPermission(
	hdlctrlv1connect.GroupServiceUpdateGroupProcedure,
	checkGroupPermission(entity.PermKey_GroupEdit, groupIDFromUpdate, false),
)

func (s *GroupService) UpdateGroup(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateGroupRequest]) (*connect.Response[hdlctrlv1.UpdateGroupResponse], error) {
	if req.Msg.Name == nil {
		// nothing to update
		g, err := s.guc.GetGroup(ctx, req.Msg.GetGroupId())
		if err != nil {
			return nil, convertErr(err)
		}

		return connect.NewResponse(&hdlctrlv1.UpdateGroupResponse{Group: groupToProto(g)}), nil
	}

	g, err := s.guc.UpdateGroupName(ctx, req.Msg.GetGroupId(), req.Msg.GetName())
	if err != nil {
		if errors.Is(err, usecase.ErrGroupOperationForbidden) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.UpdateGroupResponse{Group: groupToProto(g)}), nil
}

// DeleteGroup: group_id に対して group:edit.
var _ = registerRPCPermission(
	hdlctrlv1connect.GroupServiceDeleteGroupProcedure,
	checkGroupPermission(entity.PermKey_GroupEdit, groupIDFromDelete, false),
)

func (s *GroupService) DeleteGroup(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteGroupRequest]) (*connect.Response[hdlctrlv1.DeleteGroupResponse], error) {
	if err := s.guc.DeleteGroup(ctx, req.Msg.GetGroupId()); err != nil {
		if errors.Is(err, usecase.ErrGroupOperationForbidden) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.DeleteGroupResponse{}), nil
}

// ListGroupMembers: 所属しているか system:group.list を持つなら閲覧可.
var _ = registerRPCPermission(
	hdlctrlv1connect.GroupServiceListGroupMembersProcedure,
	checkGroupPermission(entity.PermKey_GroupEdit, groupIDFromListMembers, true),
)

func (s *GroupService) ListGroupMembers(ctx context.Context, req *connect.Request[hdlctrlv1.ListGroupMembersRequest]) (*connect.Response[hdlctrlv1.ListGroupMembersResponse], error) {
	members, err := s.guc.ListGroupMembers(ctx, req.Msg.GetGroupId())
	if err != nil {
		return nil, convertErr(err)
	}

	protoMembers := make([]*hdlctrlv1.GroupMember, 0, len(members))
	for _, m := range members {
		protoMembers = append(protoMembers, groupMemberToProto(m))
	}

	return connect.NewResponse(&hdlctrlv1.ListGroupMembersResponse{Members: protoMembers}), nil
}

// AddGroupMember: group_id に対して group:members.manage.
var _ = registerRPCPermission(
	hdlctrlv1connect.GroupServiceAddGroupMemberProcedure,
	checkGroupPermission(entity.PermKey_GroupMembersManage, groupIDFromAddMember, false),
)

func (s *GroupService) AddGroupMember(ctx context.Context, req *connect.Request[hdlctrlv1.AddGroupMemberRequest]) (*connect.Response[hdlctrlv1.AddGroupMemberResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	addedBy := claims.UserID

	m, err := s.guc.AddGroupMember(ctx, req.Msg.GetGroupId(), req.Msg.GetUserId(), req.Msg.GetRoleId(), &addedBy)
	if err != nil {
		if errors.Is(err, usecase.ErrGroupOperationForbidden) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.AddGroupMemberResponse{Member: groupMemberToProto(m)}), nil
}

// RemoveGroupMember: group_id に対して group:members.manage.
var _ = registerRPCPermission(
	hdlctrlv1connect.GroupServiceRemoveGroupMemberProcedure,
	checkGroupPermission(entity.PermKey_GroupMembersManage, groupIDFromRemoveMember, false),
)

func (s *GroupService) RemoveGroupMember(ctx context.Context, req *connect.Request[hdlctrlv1.RemoveGroupMemberRequest]) (*connect.Response[hdlctrlv1.RemoveGroupMemberResponse], error) {
	if err := s.guc.RemoveGroupMember(ctx, req.Msg.GetGroupId(), req.Msg.GetUserId()); err != nil {
		if errors.Is(err, usecase.ErrGroupOperationForbidden) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.RemoveGroupMemberResponse{}), nil
}

// UpdateGroupMemberRole: personal なら system:group.manage 必須, それ以外は group:members.manage.
var _ = registerRPCPermission(
	hdlctrlv1connect.GroupServiceUpdateGroupMemberRoleProcedure,
	checkUpdateGroupMemberRole,
)

func (s *GroupService) UpdateGroupMemberRole(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateGroupMemberRoleRequest]) (*connect.Response[hdlctrlv1.UpdateGroupMemberRoleResponse], error) {
	m, err := s.guc.UpdateGroupMemberRole(ctx, req.Msg.GetGroupId(), req.Msg.GetUserId(), req.Msg.GetRoleId())
	if err != nil {
		if errors.Is(err, usecase.ErrGroupOperationForbidden) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.UpdateGroupMemberRoleResponse{Member: groupMemberToProto(m)}), nil
}

func groupToProto(g *entity.Group) *hdlctrlv1.Group {
	p := &hdlctrlv1.Group{
		Id:   g.ID,
		Name: g.Name,
		Type: groupTypeToProto(g.Type),
	}

	if !g.CreatedAt.IsZero() {
		p.CreatedAt = timestamppb.New(g.CreatedAt)
	}

	if !g.UpdatedAt.IsZero() {
		p.UpdatedAt = timestamppb.New(g.UpdatedAt)
	}

	return p
}

func groupTypeToProto(t entity.GroupType) hdlctrlv1.GroupType {
	switch t {
	case entity.GroupType_Personal:
		return hdlctrlv1.GroupType_GROUP_TYPE_PERSONAL
	case entity.GroupType_Normal:
		return hdlctrlv1.GroupType_GROUP_TYPE_NORMAL
	case entity.GroupType_System:
		return hdlctrlv1.GroupType_GROUP_TYPE_SYSTEM
	}

	return hdlctrlv1.GroupType_GROUP_TYPE_UNSPECIFIED
}

func groupMemberToProto(m *entity.GroupMember) *hdlctrlv1.GroupMember {
	p := &hdlctrlv1.GroupMember{
		GroupId: m.GroupID,
		UserId:  m.UserID,
		RoleId:  m.RoleID,
		AddedBy: m.AddedBy,
	}
	if !m.JoinedAt.IsZero() {
		p.JoinedAt = timestamppb.New(m.JoinedAt)
	}

	return p
}
