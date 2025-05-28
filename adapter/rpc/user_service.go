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

func NewUserService(uu *usecase.UserUsecase) *UserService {
	return &UserService{
		uu: uu,
	}
}

func (u *UserService) NewHandler() (string, http.Handler) {
	interceptors := connect.WithInterceptors(logging.NewErrorLogInterceptor())
	return hdlctrlv1connect.NewUserServiceHandler(u, interceptors)
}
