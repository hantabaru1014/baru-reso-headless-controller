// role_service.go は RoleService の connect RPC handler.
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

var _ hdlctrlv1connect.RoleServiceHandler = (*RoleService)(nil)

type RoleService struct {
	ruc       *usecase.RoleUsecase
	permUC    *usecase.PermissionUsecase
	groupRepo port.GroupRepository
	roleRepo  port.RoleRepository

	// permission interceptor 依存
	hostRepo    port.HeadlessHostRepository
	sessionRepo port.SessionRepository
	hauc        *usecase.HeadlessAccountUsecase
}

func NewRoleService(
	ruc *usecase.RoleUsecase,
	permUC *usecase.PermissionUsecase,
	groupRepo port.GroupRepository,
	roleRepo port.RoleRepository,
	hostRepo port.HeadlessHostRepository,
	sessionRepo port.SessionRepository,
	hauc *usecase.HeadlessAccountUsecase,
) *RoleService {
	return &RoleService{
		ruc:         ruc,
		permUC:      permUC,
		groupRepo:   groupRepo,
		roleRepo:    roleRepo,
		hostRepo:    hostRepo,
		sessionRepo: sessionRepo,
		hauc:        hauc,
	}
}

func (s *RoleService) NewHandler() (string, http.Handler) {
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

	return hdlctrlv1connect.NewRoleServiceHandler(s, interceptors)
}

// ListRoles: 認証のみ (handler 側でグローバル + 指定グループのカスタムを返す).
var _ = registerRPCPermission(
	hdlctrlv1connect.RoleServiceListRolesProcedure,
	requireAuthOnly,
)

func (s *RoleService) ListRoles(ctx context.Context, req *connect.Request[hdlctrlv1.ListRolesRequest]) (*connect.Response[hdlctrlv1.ListRolesResponse], error) {
	var groupID *string

	if req.Msg.GroupId != nil && *req.Msg.GroupId != "" {
		g := req.Msg.GetGroupId()
		groupID = &g
	}

	roles, err := s.ruc.ListRoles(ctx, groupID)
	if err != nil {
		return nil, convertErr(err)
	}

	out := make([]*hdlctrlv1.Role, 0, len(roles))
	for _, r := range roles {
		out = append(out, roleToProto(r))
	}

	return connect.NewResponse(&hdlctrlv1.ListRolesResponse{Roles: out}), nil
}

// CreateRole: group_id 指定なら group:members.manage, 未指定なら system:role.manage.
var _ = registerRPCPermission(
	hdlctrlv1connect.RoleServiceCreateRoleProcedure,
	checkCreateRole,
)

func (s *RoleService) CreateRole(ctx context.Context, req *connect.Request[hdlctrlv1.CreateRoleRequest]) (*connect.Response[hdlctrlv1.CreateRoleResponse], error) {
	var groupID *string

	if req.Msg.GroupId != nil && *req.Msg.GroupId != "" {
		g := req.Msg.GetGroupId()
		groupID = &g
	}

	role, err := s.ruc.CreateRole(ctx, usecase.CreateRoleParams{
		GroupID:        groupID,
		Name:           req.Msg.GetName(),
		Scope:          protoRoleScopeToEntity(req.Msg.GetScope()),
		PermissionKeys: req.Msg.GetPermissionKeys(),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidPermissionKey) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.CreateRoleResponse{Role: roleToProto(role)}), nil
}

// UpdateRole: 対象 role がグローバルなら system:role.manage, グループ内なら group:members.manage.
var _ = registerRPCPermission(
	hdlctrlv1connect.RoleServiceUpdateRoleProcedure,
	checkUpdateRole,
)

func (s *RoleService) UpdateRole(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateRoleRequest]) (*connect.Response[hdlctrlv1.UpdateRoleResponse], error) {
	params := usecase.UpdateRoleParams{
		ID:   req.Msg.GetRoleId(),
		Name: req.Msg.Name,
	}

	if req.Msg.PermissionKeys != nil {
		keys := req.Msg.GetPermissionKeys().GetKeys()
		params.PermissionKeys = &keys
		params.OverwritePermKeys = true
	}

	role, err := s.ruc.UpdateRole(ctx, params)
	if err != nil {
		if errors.Is(err, usecase.ErrCannotModifyBuiltinRole) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		if errors.Is(err, usecase.ErrInvalidPermissionKey) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.UpdateRoleResponse{Role: roleToProto(role)}), nil
}

// DeleteRole: 対象 role がグローバルなら system:role.manage, グループ内なら group:members.manage.
var _ = registerRPCPermission(
	hdlctrlv1connect.RoleServiceDeleteRoleProcedure,
	checkDeleteRole,
)

func (s *RoleService) DeleteRole(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteRoleRequest]) (*connect.Response[hdlctrlv1.DeleteRoleResponse], error) {
	if err := s.ruc.DeleteRole(ctx, req.Msg.GetRoleId()); err != nil {
		if errors.Is(err, usecase.ErrCannotModifyBuiltinRole) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.DeleteRoleResponse{}), nil
}

// ListPermissions: 認証のみ (利用可能 permission_key 一覧は機密でない).
var _ = registerRPCPermission(
	hdlctrlv1connect.RoleServiceListPermissionsProcedure,
	requireAuthOnly,
)

func (s *RoleService) ListPermissions(_ context.Context, req *connect.Request[hdlctrlv1.ListPermissionsRequest]) (*connect.Response[hdlctrlv1.ListPermissionsResponse], error) {
	scope := protoRoleScopeToEntity(req.Msg.GetScope())

	perms := make([]*hdlctrlv1.PermissionKey, 0, len(entity.AllPermissionKeys))
	for _, p := range entity.AllPermissionKeys {
		if scope != "" && p.Scope != scope {
			continue
		}

		perms = append(perms, &hdlctrlv1.PermissionKey{
			Key:         p.Key,
			Description: p.Description,
			Scope:       entityRoleScopeToProto(p.Scope),
		})
	}

	return connect.NewResponse(&hdlctrlv1.ListPermissionsResponse{Permissions: perms}), nil
}

// GetMyPermissions: 認証のみ (自分の権限サマリーを取得).
var _ = registerRPCPermission(
	hdlctrlv1connect.RoleServiceGetMyPermissionsProcedure,
	requireAuthOnly,
)

func (s *RoleService) GetMyPermissions(ctx context.Context, _ *connect.Request[hdlctrlv1.GetMyPermissionsRequest]) (*connect.Response[hdlctrlv1.GetMyPermissionsResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	summary, err := s.permUC.GetMyPermissionsSummary(ctx, claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	out := &hdlctrlv1.MyPermissions{
		Groups:               make([]*hdlctrlv1.GroupPermissions, 0, len(summary.Groups)),
		SystemPermissionKeys: summary.SystemPermissionKeys,
	}
	for _, g := range summary.Groups {
		out.Groups = append(out.Groups, &hdlctrlv1.GroupPermissions{
			GroupId:        g.GroupID,
			RoleId:         g.RoleID,
			PermissionKeys: g.PermissionKeys,
		})
	}

	return connect.NewResponse(&hdlctrlv1.GetMyPermissionsResponse{Permissions: out}), nil
}

func roleToProto(r *entity.Role) *hdlctrlv1.Role {
	p := &hdlctrlv1.Role{
		Id:             r.ID,
		GroupId:        r.GroupID,
		Name:           r.Name,
		Scope:          entityRoleScopeToProto(r.Scope),
		IsBuiltin:      r.IsBuiltin,
		PermissionKeys: r.PermissionKeys,
	}
	if !r.CreatedAt.IsZero() {
		p.CreatedAt = timestamppb.New(r.CreatedAt)
	}

	if !r.UpdatedAt.IsZero() {
		p.UpdatedAt = timestamppb.New(r.UpdatedAt)
	}

	return p
}

func entityRoleScopeToProto(s entity.RoleScope) hdlctrlv1.RoleScope {
	switch s {
	case entity.RoleScope_Normal:
		return hdlctrlv1.RoleScope_ROLE_SCOPE_NORMAL
	case entity.RoleScope_System:
		return hdlctrlv1.RoleScope_ROLE_SCOPE_SYSTEM
	}

	return hdlctrlv1.RoleScope_ROLE_SCOPE_UNSPECIFIED
}

func protoRoleScopeToEntity(s hdlctrlv1.RoleScope) entity.RoleScope {
	switch s {
	case hdlctrlv1.RoleScope_ROLE_SCOPE_NORMAL:
		return entity.RoleScope_Normal
	case hdlctrlv1.RoleScope_ROLE_SCOPE_SYSTEM:
		return entity.RoleScope_System
	}

	return ""
}
