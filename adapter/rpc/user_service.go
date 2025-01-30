package rpc

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
)

var _ hdlctrlv1connect.UserServiceHandler = (*UserService)(nil)

var apiKey = os.Getenv("API_KEY")
var userCredentials = strings.Split(os.Getenv("USER_CREDENTIALS"), ",")

type UserService struct {
	uu *usecase.UserUsecase
}

// RefreshToken implements hdlctrlv1connect.UserServiceHandler.
func (u *UserService) RefreshToken(ctx context.Context, req *connect.Request[hdlctrlv1.RefreshTokenRequest]) (*connect.Response[hdlctrlv1.TokenSetResponse], error) {
	claims, err := auth.ValidateToken(ctx, req)
	if err != nil {
		return nil, err
	}

	token, refreshToken, err := auth.GenerateTokensWithDefaultTTL(*claims)
	if err != nil {
		return nil, err
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
	for _, cred := range userCredentials {
		parts := strings.Split(cred, ":")
		if req.Msg.Id == parts[1] && req.Msg.Password == parts[2] {
			token, refreshToken, err := auth.GenerateTokensWithDefaultTTL(auth.AuthClaims{
				UserID: parts[0],
			})
			if err != nil {
				return nil, err
			}
			res := connect.NewResponse(&hdlctrlv1.TokenSetResponse{
				Token:        token,
				RefreshToken: refreshToken,
			})
			return res, nil
		}
	}

	return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id or password"))
}

// GetTokenByAPIKey implements hdlctrlv1connect.UserServiceHandler.
func (u *UserService) GetTokenByAPIKey(ctx context.Context, req *connect.Request[hdlctrlv1.GetTokenByAPIKeyRequest]) (*connect.Response[hdlctrlv1.TokenSetResponse], error) {
	if req.Msg.ApiKey != apiKey {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid api key"))
	}

	token, refreshToken, err := auth.GenerateTokensWithDefaultTTL(auth.AuthClaims{
		UserID: "admin",
	})
	if err != nil {
		return nil, err
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
	return hdlctrlv1connect.NewUserServiceHandler(u)
}
