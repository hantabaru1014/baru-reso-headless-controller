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
)

var _ hdlctrlv1connect.UserServiceHandler = (*UserService)(nil)

var apiKey = os.Getenv("API_KEY")
var userCredentials = strings.Split(os.Getenv("USER_CREDENTIALS"), ",")

type UserService struct{}

// GetTokenByPassword implements hdlctrlv1connect.UserServiceHandler.
func (u *UserService) GetTokenByPassword(ctx context.Context, req *connect.Request[hdlctrlv1.GetTokenByPasswordRequest]) (*connect.Response[hdlctrlv1.GetTokenByPasswordResponse], error) {
	for _, cred := range userCredentials {
		parts := strings.Split(cred, ":")
		if req.Msg.Id == parts[1] && req.Msg.Password == parts[2] {
			token, err := auth.GenerateToken(auth.AuthClaims{
				UserID: parts[0],
			}, true)
			if err != nil {
				return nil, err
			}
			res := connect.NewResponse(&hdlctrlv1.GetTokenByPasswordResponse{
				Token: token,
			})
			return res, nil
		}
	}

	return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id or password"))
}

// GetTokenByAPIKey implements hdlctrlv1connect.UserServiceHandler.
func (u *UserService) GetTokenByAPIKey(ctx context.Context, req *connect.Request[hdlctrlv1.GetTokenByAPIKeyRequest]) (*connect.Response[hdlctrlv1.GetTokenByAPIKeyResponse], error) {
	if req.Msg.ApiKey != apiKey {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid api key"))
	}

	token, err := auth.GenerateToken(auth.AuthClaims{
		UserID: "admin",
	}, true)
	if err != nil {
		return nil, err
	}
	res := connect.NewResponse(&hdlctrlv1.GetTokenByAPIKeyResponse{
		Token: token,
	})
	return res, nil
}

func NewUserService() *UserService {
	return &UserService{}
}

func (u *UserService) NewHandler() (string, http.Handler) {
	return hdlctrlv1connect.NewUserServiceHandler(u)
}
