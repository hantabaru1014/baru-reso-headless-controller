package rpc

import (
	"context"
	"errors"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
)

var _ hdlctrlv1connect.UserServiceHandler = (*UserService)(nil)

var apiKey = os.Getenv("API_KEY")

type UserService struct{}

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

func (u *UserService) Handle(mux *http.ServeMux) {
	path, handler := hdlctrlv1connect.NewUserServiceHandler(u)
	mux.Handle(path, handler)
}
