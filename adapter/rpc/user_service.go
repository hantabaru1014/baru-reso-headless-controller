package rpc

import (
	"context"
	"net/http"

	"github.com/go-errors/errors"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/logging"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
)

var _ hdlctrlv1connect.UserServiceHandler = (*UserService)(nil)

type UserService struct {
	uu *usecase.UserUsecase
}

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
	user, err := u.uu.GetUserWithPassword(ctx, req.Msg.Id, req.Msg.Password)
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
	userInfo, err := u.uu.ValidateRegistrationToken(ctx, req.Msg.Token)
	if err != nil {
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
	user, err := u.uu.RegisterWithToken(ctx, req.Msg.Token, req.Msg.UserId, req.Msg.Password)
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
	err = u.uu.ChangePassword(ctx, claims.UserID, req.Msg.CurrentPassword, req.Msg.NewPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&hdlctrlv1.ChangePasswordResponse{}), nil
}

func NewUserService(uu *usecase.UserUsecase) *UserService {
	return &UserService{
		uu: uu,
	}
}

func (u *UserService) NewHandler() (string, http.Handler) {
	interceptors := connect.WithInterceptors(
		logging.NewErrorLogInterceptor(),
		auth.NewOptionalAuthInterceptor(),
	)
	return hdlctrlv1connect.NewUserServiceHandler(u, interceptors)
}
