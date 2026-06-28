package rpc

import (
	"context"
	"net/http"

	"github.com/go-errors/errors"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/logging"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ hdlctrlv1connect.UserServiceHandler = (*UserService)(nil)

type UserService struct {
	uu     *usecase.UserUsecase
	permUC *usecase.PermissionUsecase
}

func NewUserService(uu *usecase.UserUsecase, permUC *usecase.PermissionUsecase) *UserService {
	return &UserService{
		uu:     uu,
		permUC: permUC,
	}
}

// 公開 / 自己オペレーション RPC は permission_interceptor の fail-closed default を
// 通過するため明示的に publicRPC / requireAuthenticated で登録する.
var (
	_ = registerRPCPermission(hdlctrlv1connect.UserServiceGetTokenByPasswordProcedure, publicRPC)
	_ = registerRPCPermission(hdlctrlv1connect.UserServiceValidateRegistrationTokenProcedure, publicRPC)
	_ = registerRPCPermission(hdlctrlv1connect.UserServiceRegisterWithTokenProcedure, publicRPC)
	// RefreshToken は Bearer ヘッダの refresh token を handler で検証するため
	// 認証は handler 側でやる. ここでは public 扱い.
	_ = registerRPCPermission(hdlctrlv1connect.UserServiceRefreshTokenProcedure, publicRPC)
	// ChangePassword は access token 必須 (handler でも claims を参照する).
	_ = registerRPCPermission(hdlctrlv1connect.UserServiceChangePasswordProcedure, requireAuthenticated)
)

// RefreshToken implements hdlctrlv1connect.UserServiceHandler.
func (u *UserService) RefreshToken(ctx context.Context, req *connect.Request[hdlctrlv1.RefreshTokenRequest]) (*connect.Response[hdlctrlv1.TokenSetResponse], error) {
	claims, err := auth.ValidateToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	token, refreshToken, err := auth.GenerateTokensWithDefaultTTL(*claims)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	res := connect.NewResponse(&hdlctrlv1.TokenSetResponse{
		Token:        token,
		RefreshToken: refreshToken,
	})
	auth.SetSuccessResponseHeader(res)

	return res, nil
}

// GetTokenByPassword implements hdlctrlv1connect.UserServiceHandler.
func (u *UserService) GetTokenByPassword(ctx context.Context, req *connect.Request[hdlctrlv1.GetTokenByPasswordRequest]) (*connect.Response[hdlctrlv1.TokenSetResponse], error) {
	user, err := u.uu.GetUserWithPassword(ctx, req.Msg.GetId(), req.Msg.GetPassword())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id or password"))
	}

	token, refreshToken, err := auth.GenerateTokensWithDefaultTTL(auth.AuthClaims{
		UserID:     user.ID,
		ResoniteID: user.ResoniteID.String,
		IconUrl:    user.IconUrl.String,
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	res := connect.NewResponse(&hdlctrlv1.TokenSetResponse{
		Token:        token,
		RefreshToken: refreshToken,
	})

	return res, nil
}

// ValidateRegistrationToken implements hdlctrlv1connect.UserServiceHandler.
func (u *UserService) ValidateRegistrationToken(ctx context.Context, req *connect.Request[hdlctrlv1.ValidateRegistrationTokenRequest]) (*connect.Response[hdlctrlv1.ValidateRegistrationTokenResponse], error) {
	userInfo, err := u.uu.ValidateRegistrationToken(ctx, req.Msg.GetToken())
	if err != nil {
		// トークンが無効な場合はエラーを返さずValid=falseを返す
		//nolint:nilerr // intentional: return Valid=false instead of error for invalid token
		return connect.NewResponse(&hdlctrlv1.ValidateRegistrationTokenResponse{
			Valid:            false,
			ResoniteId:       "",
			ResoniteUserName: "",
			IconUrl:          "",
		}), nil
	}

	return connect.NewResponse(&hdlctrlv1.ValidateRegistrationTokenResponse{
		Valid:            true,
		ResoniteId:       userInfo.ID,
		ResoniteUserName: userInfo.UserName,
		IconUrl:          userInfo.IconUrl,
	}), nil
}

// RegisterWithToken implements hdlctrlv1connect.UserServiceHandler.
func (u *UserService) RegisterWithToken(ctx context.Context, req *connect.Request[hdlctrlv1.RegisterWithTokenRequest]) (*connect.Response[hdlctrlv1.TokenSetResponse], error) {
	user, err := u.uu.RegisterWithToken(ctx, req.Msg.GetToken(), req.Msg.GetUserId(), req.Msg.GetPassword())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("registration failed: invalid token or user already exists"))
	}

	token, refreshToken, err := auth.GenerateTokensWithDefaultTTL(auth.AuthClaims{
		UserID:     user.ID,
		ResoniteID: user.ResoniteID.String,
		IconUrl:    user.IconUrl.String,
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return connect.NewResponse(&hdlctrlv1.TokenSetResponse{
		Token:        token,
		RefreshToken: refreshToken,
	}), nil
}

// ChangePassword implements hdlctrlv1connect.UserServiceHandler.
func (u *UserService) ChangePassword(ctx context.Context, req *connect.Request[hdlctrlv1.ChangePasswordRequest]) (*connect.Response[hdlctrlv1.ChangePasswordResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	err = u.uu.ChangePassword(ctx, claims.UserID, req.Msg.GetCurrentPassword(), req.Msg.GetNewPassword())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&hdlctrlv1.ChangePasswordResponse{}), nil
}

// ListUsers implements hdlctrlv1connect.UserServiceHandler.
// 権限: system:user.list (permission interceptor が事前チェック).
var _ = registerRPCPermission(
	hdlctrlv1connect.UserServiceListUsersProcedure,
	requireSystemPerm(entity.PermKey_SystemUserList),
)

func (u *UserService) ListUsers(ctx context.Context, _ *connect.Request[hdlctrlv1.ListUsersRequest]) (*connect.Response[hdlctrlv1.ListUsersResponse], error) {
	users, err := u.uu.ListUsers(ctx)
	if err != nil {
		return nil, convertErr(err)
	}

	protoUsers := make([]*hdlctrlv1.User, 0, len(users))
	for i := range users {
		protoUsers = append(protoUsers, userToProto(&users[i]))
	}

	return connect.NewResponse(&hdlctrlv1.ListUsersResponse{Users: protoUsers}), nil
}

// GetUser implements hdlctrlv1connect.UserServiceHandler.
// 権限: 認証済みなら誰でも (グループメンバー名解決等の汎用用途).
var _ = registerRPCPermission(
	hdlctrlv1connect.UserServiceGetUserProcedure,
	requireAuthenticated,
)

func (u *UserService) GetUser(ctx context.Context, req *connect.Request[hdlctrlv1.GetUserRequest]) (*connect.Response[hdlctrlv1.GetUserResponse], error) {
	if req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}

	user, err := u.uu.GetUser(ctx, req.Msg.GetUserId())
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.GetUserResponse{User: userToProto(user)}), nil
}

// CreateRegistrationToken implements hdlctrlv1connect.UserServiceHandler.
// 権限: system:user.create.
var _ = registerRPCPermission(
	hdlctrlv1connect.UserServiceCreateRegistrationTokenProcedure,
	requireSystemPerm(entity.PermKey_SystemUserCreate),
)

func (u *UserService) CreateRegistrationToken(ctx context.Context, req *connect.Request[hdlctrlv1.CreateRegistrationTokenRequest]) (*connect.Response[hdlctrlv1.CreateRegistrationTokenResponse], error) {
	if req.Msg.GetResoniteId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("resonite_id is required"))
	}

	info, err := u.uu.CreateRegistrationTokenWithInfo(ctx, req.Msg.GetResoniteId())
	if err != nil {
		// Resonite ID 不正は InvalidArgument として扱う.
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&hdlctrlv1.CreateRegistrationTokenResponse{
		Token:            info.Token,
		ExpiresAt:        timestamppb.New(info.ExpiresAt),
		ResoniteUserName: info.ResoniteUserName,
		IconUrl:          info.IconUrl,
	}), nil
}

// DeleteUser implements hdlctrlv1connect.UserServiceHandler.
// 権限: system:user.delete. 自分自身は削除不可.
var _ = registerRPCPermission(
	hdlctrlv1connect.UserServiceDeleteUserProcedure,
	requireSystemPerm(entity.PermKey_SystemUserDelete),
)

func (u *UserService) DeleteUser(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteUserRequest]) (*connect.Response[hdlctrlv1.DeleteUserResponse], error) {
	if req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}

	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// 自己削除ガード: 自分自身は削除できない (system-admin が誤って自分を消す事故を防ぐ).
	if claims.UserID == req.Msg.GetUserId() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot delete yourself"))
	}

	if err := u.uu.DeleteUser(ctx, req.Msg.GetUserId()); err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.DeleteUserResponse{}), nil
}

func userToProto(u *db.User) *hdlctrlv1.User {
	p := &hdlctrlv1.User{
		Id:         u.ID,
		ResoniteId: u.ResoniteID.String,
		IconUrl:    u.IconUrl.String,
	}

	if u.CreatedAt.Valid {
		p.CreatedAt = timestamppb.New(u.CreatedAt.Time)
	}

	if u.UpdatedAt.Valid {
		p.UpdatedAt = timestamppb.New(u.UpdatedAt.Time)
	}

	return p
}

func (u *UserService) NewHandler() (string, http.Handler) {
	interceptors := connect.WithInterceptors(
		logging.NewErrorLogInterceptor(),
		auth.NewOptionalAuthInterceptor(),
		// 管理用 RPC (ListUsers / GetUser / CreateRegistrationToken / DeleteUser) の
		// 権限チェック. 公開 RPC は rpcPermissionRules に登録されていないので pass-through.
		NewPermissionInterceptor(u.permUC, PermissionDeps{}),
	)

	return hdlctrlv1connect.NewUserServiceHandler(u, interceptors)
}
