package rpc

import (
	"context"
	"encoding/json"
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

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// ChangePasswordHandler はパスワード変更用のHTTPハンドラーを返す
func (u *UserService) ChangePasswordHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// 認証トークンを検証
		claims, err := auth.ValidateTokenFromHeader(r.Header.Get("Authorization"))
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(errorResponse{Error: "unauthorized"})
			return
		}

		// リクエストボディをパース
		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"})
			return
		}

		// パスワードを更新
		if err := u.uu.UpdatePassword(r.Context(), claims.UserID, req.CurrentPassword, req.NewPassword); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorResponse{Error: "invalid current password"})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
