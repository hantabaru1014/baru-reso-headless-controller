// Package rpc は ControllerService / GroupService / RoleService / UserService 等の
// connect RPC handler を提供する.
//
// permission_interceptor.go は connect.Interceptor として、認証 interceptor の
// 後段で動作する権限チェック層. procedure 名ごとに必要 permission_key と
// 「group_id の解決方法」を rpcPermissionRules に登録し、リクエストから対象
// リソースの group_id を引いて user の permission を判定する.
//
// 仕様: docs/permissions.md 5.2.
package rpc

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// PermissionDeps は interceptor が group_id 解決に使う依存.
type PermissionDeps struct {
	HostRepo    port.HeadlessHostRepository
	SessionRepo port.SessionRepository
	AccountUC   *usecase.HeadlessAccountUsecase
	GroupRepo   port.GroupRepository
	RoleRepo    port.RoleRepository
}

// permissionCheck は 1 つの RPC についての permission 判定ロジック.
// 戻り値 error は connect.Error をラップ済みのものを返す.
type permissionCheck func(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error

// rpcPermissionRules は procedure 名 → permissionCheck マッピング.
// 各 controller_service_*.go / group_service.go / role_service.go / user_service.go の
// ハンドラ宣言の直前に置かれた `var _ = registerRPCPermission(procedure, check)` から
// 自動的に登録される (グローバル変数初期化は init() より先に走るため安全).
//
// 全 procedure 登録の有無は init() で fail-fast に検証する.
var rpcPermissionRules = map[string]permissionCheck{}

// registerRPCPermission は各 service ファイルから自ファイル内の RPC 権限ルールを
// 宣言するためのヘルパー. var _ = registerRPCPermission(...) の形で使う.
//
// 同一 procedure を二重登録した場合は panic で初期化を止める (誤った宣言を
// fail-fast に検知するため).
func registerRPCPermission(procedure string, check permissionCheck) struct{} { //nolint:unparam // 戻り値は `var _ = registerRPCPermission(...)` を file-scope で書くためのダミー
	if rpcPermissionRules == nil {
		rpcPermissionRules = map[string]permissionCheck{}
	}

	if _, exists := rpcPermissionRules[procedure]; exists {
		panic("duplicate rpc permission registration: " + procedure)
	}

	rpcPermissionRules[procedure] = check

	return struct{}{}
}

// init は全 procedure に permission ルールが登録されているかを検証する.
// 不足があれば panic で server 起動を止める (実装ミスの fail-fast).
//
// 期待する procedure 一覧は allKnownProcedures で手動列挙する.
// proto に新規 RPC を追加した場合はこの一覧と register 側の両方を更新する必要があり,
// 二重チェックとして働く.
func init() { //nolint:gochecknoinits // 登録漏れ検知.
	for _, p := range allKnownProcedures() {
		if _, ok := rpcPermissionRules[p]; !ok {
			panic("missing rpc permission registration: " + p)
		}
	}
}

// allKnownProcedures は permission interceptor を通過する全 procedure 名を返す.
// 新規 RPC 追加時はここに追記する (= 登録漏れ検知の対象に加える).
//
// UserService の未認証 RPC (GetTokenByPassword / ValidateRegistrationToken /
// RegisterWithToken / RefreshToken / ChangePassword) も含めるかどうかは方針次第.
// 現状: ChangePassword は handler 側で claims を要求し、それ以外は完全に公開.
// 公開 RPC は意図的に登録不要 (handler 側で claim 取得しないため) と区別するため、
// allKnownProcedures には permission interceptor で扱う procedure のみを列挙する.
func allKnownProcedures() []string {
	return []string{
		// ===== ControllerService: ホスト系 =====
		hdlctrlv1connect.ControllerServiceListHeadlessHostProcedure,
		hdlctrlv1connect.ControllerServiceGetHeadlessHostProcedure,
		hdlctrlv1connect.ControllerServiceGetHeadlessHostLogsProcedure,
		hdlctrlv1connect.ControllerServiceListHeadlessHostInstancesProcedure,
		hdlctrlv1connect.ControllerServiceShutdownHeadlessHostProcedure,
		hdlctrlv1connect.ControllerServiceKillHeadlessHostProcedure,
		hdlctrlv1connect.ControllerServiceUpdateHeadlessHostSettingsProcedure,
		hdlctrlv1connect.ControllerServiceRestartHeadlessHostProcedure,
		hdlctrlv1connect.ControllerServiceDeleteHeadlessHostProcedure,
		hdlctrlv1connect.ControllerServiceAllowHostAccessProcedure,
		hdlctrlv1connect.ControllerServiceDenyHostAccessProcedure,
		hdlctrlv1connect.ControllerServiceListHeadlessHostImageTagsProcedure,
		hdlctrlv1connect.ControllerServiceStartHeadlessHostProcedure,

		// ===== ControllerService: アカウント系 =====
		hdlctrlv1connect.ControllerServiceListHeadlessAccountsProcedure,
		hdlctrlv1connect.ControllerServiceCreateHeadlessAccountProcedure,
		hdlctrlv1connect.ControllerServiceDeleteHeadlessAccountProcedure,
		hdlctrlv1connect.ControllerServiceUpdateHeadlessAccountCredentialsProcedure,
		hdlctrlv1connect.ControllerServiceGetHeadlessAccountStorageInfoProcedure,
		hdlctrlv1connect.ControllerServiceRefetchHeadlessAccountInfoProcedure,
		hdlctrlv1connect.ControllerServiceUpdateHeadlessAccountIconProcedure,

		// ===== ControllerService: Cloud系 =====
		hdlctrlv1connect.ControllerServiceFetchWorldInfoProcedure,
		hdlctrlv1connect.ControllerServiceSearchUserInfoProcedure,
		hdlctrlv1connect.ControllerServiceSearchWorldsProcedure,
		hdlctrlv1connect.ControllerServiceGetOwnWorldsProcedure,
		hdlctrlv1connect.ControllerServiceGetResoniteUserProcedure,
		hdlctrlv1connect.ControllerServiceGetFriendRequestsProcedure,
		hdlctrlv1connect.ControllerServiceAcceptFriendRequestsProcedure,
		hdlctrlv1connect.ControllerServiceSendFriendRequestProcedure,
		hdlctrlv1connect.ControllerServiceRemoveContactProcedure,

		// ===== ControllerService: コンタクト・チャット系 =====
		hdlctrlv1connect.ControllerServiceListContactsProcedure,
		hdlctrlv1connect.ControllerServiceGetContactMessagesProcedure,
		hdlctrlv1connect.ControllerServiceSendContactMessageProcedure,

		// ===== ControllerService: セッション系 =====
		hdlctrlv1connect.ControllerServiceSearchSessionsProcedure,
		hdlctrlv1connect.ControllerServiceGetSessionDetailsProcedure,
		hdlctrlv1connect.ControllerServiceStartWorldProcedure,
		hdlctrlv1connect.ControllerServiceStopSessionProcedure,
		hdlctrlv1connect.ControllerServiceDeleteEndedSessionProcedure,
		hdlctrlv1connect.ControllerServiceSaveSessionWorldProcedure,
		hdlctrlv1connect.ControllerServicePrepareSessionWorldDownloadProcedure,
		hdlctrlv1connect.ControllerServiceInviteUserProcedure,
		hdlctrlv1connect.ControllerServiceUpdateUserRoleProcedure,
		hdlctrlv1connect.ControllerServiceUpdateSessionParametersProcedure,
		hdlctrlv1connect.ControllerServiceUpdateSessionExtraSettingsProcedure,
		hdlctrlv1connect.ControllerServiceListUsersInSessionProcedure,
		hdlctrlv1connect.ControllerServiceKickUserProcedure,
		hdlctrlv1connect.ControllerServiceBanUserProcedure,
		hdlctrlv1connect.ControllerServiceListBansProcedure,
		hdlctrlv1connect.ControllerServiceUnbanUserProcedure,
		hdlctrlv1connect.ControllerServiceRespawnUserProcedure,
		hdlctrlv1connect.ControllerServiceSpawnItemProcedure,
		hdlctrlv1connect.ControllerServiceSendDynamicImpulseProcedure,
		hdlctrlv1connect.ControllerServiceIssueResoniteLinkConnectionProcedure,

		// ===== ControllerService: 予約操作系 =====
		hdlctrlv1connect.ControllerServiceCreateScheduledSessionOperationProcedure,
		hdlctrlv1connect.ControllerServiceListScheduledSessionOperationsProcedure,
		hdlctrlv1connect.ControllerServiceCancelScheduledSessionOperationProcedure,

		// ===== GroupService =====
		hdlctrlv1connect.GroupServiceCreateGroupProcedure,
		hdlctrlv1connect.GroupServiceGetGroupProcedure,
		hdlctrlv1connect.GroupServiceListGroupsProcedure,
		hdlctrlv1connect.GroupServiceUpdateGroupProcedure,
		hdlctrlv1connect.GroupServiceDeleteGroupProcedure,
		hdlctrlv1connect.GroupServiceListGroupMembersProcedure,
		hdlctrlv1connect.GroupServiceAddGroupMemberProcedure,
		hdlctrlv1connect.GroupServiceRemoveGroupMemberProcedure,
		hdlctrlv1connect.GroupServiceUpdateGroupMemberRoleProcedure,

		// ===== RoleService =====
		hdlctrlv1connect.RoleServiceListRolesProcedure,
		hdlctrlv1connect.RoleServiceCreateRoleProcedure,
		hdlctrlv1connect.RoleServiceUpdateRoleProcedure,
		hdlctrlv1connect.RoleServiceDeleteRoleProcedure,
		hdlctrlv1connect.RoleServiceListPermissionsProcedure,
		hdlctrlv1connect.RoleServiceGetMyPermissionsProcedure,

		// ===== UserService (管理用 RPC) =====
		hdlctrlv1connect.UserServiceListUsersProcedure,
		hdlctrlv1connect.UserServiceGetUserProcedure,
		hdlctrlv1connect.UserServiceCreateRegistrationTokenProcedure,
		hdlctrlv1connect.UserServiceDeleteUserProcedure,

		// ===== UserService (公開 RPC: 認証不要 or refresh token 経由) =====
		// fail-closed default では明示登録が必要.
		hdlctrlv1connect.UserServiceGetTokenByPasswordProcedure,
		hdlctrlv1connect.UserServiceValidateRegistrationTokenProcedure,
		hdlctrlv1connect.UserServiceRegisterWithTokenProcedure,
		hdlctrlv1connect.UserServiceRefreshTokenProcedure,
		hdlctrlv1connect.UserServiceChangePasswordProcedure,
	}
}

// permissionInterceptor は connect の unary interceptor として permission チェックを行う.
// streaming は本 OSS では permission を要する用途が現状ないため pass-through.
type permissionInterceptor struct {
	permUC *usecase.PermissionUsecase
	deps   PermissionDeps
}

func NewPermissionInterceptor(permUC *usecase.PermissionUsecase, deps PermissionDeps) connect.Interceptor {
	return &permissionInterceptor{permUC: permUC, deps: deps}
}

func (i *permissionInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := req.Spec().Procedure

		check, ok := rpcPermissionRules[procedure]
		if !ok {
			// fail-closed: 未登録 procedure は明示的に拒否する. 新規 RPC 追加時に
			// registerRPCPermission の登録漏れがあれば、init() の整合性チェックも
			// 抜けた場合の最終ガードとして PermissionDenied で弾く.
			return nil, connect.NewError(connect.CodePermissionDenied,
				errors.New("rpc has no permission rule: "+procedure))
		}

		if err := check(ctx, req, &i.deps, i.permUC); err != nil {
			return nil, err
		}

		return next(ctx, req)
	}
}

func (i *permissionInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *permissionInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// requireAuthOnly は permission チェック不要 (auth interceptor のみ).
func requireAuthOnly(_ context.Context, _ connect.AnyRequest, _ *PermissionDeps, _ *usecase.PermissionUsecase) error {
	return nil
}

// publicRPC は認証不要 RPC を明示的に許可する rule.
// fail-closed default の例外として、未認証で呼べる RPC (token 発行 / 登録 等) に
// 使う. requireAuthOnly と異なり claims の存在を期待しない.
func publicRPC(_ context.Context, _ connect.AnyRequest, _ *PermissionDeps, _ *usecase.PermissionUsecase) error {
	return nil
}

// requireAuthenticated は claims が存在することのみを確認する.
// UserService のように OptionalAuthInterceptor 配下の RPC で、認証は要るが
// 特別な権限は要らないケースで使う (例: GetUser はメンバー名解決等の用途で
// 認証ユーザーなら誰でも引ける).
func requireAuthenticated(ctx context.Context, _ connect.AnyRequest, _ *PermissionDeps, _ *usecase.PermissionUsecase) error {
	_, err := extractClaims(ctx)

	return err
}

// requireSystemPerm は system:* 権限を持つことを要求する汎用 check を返す.
// UserService の system:user.* / 将来追加される system:* 系 RPC で使う.
func requireSystemPerm(permKey string) permissionCheck {
	return func(ctx context.Context, _ connect.AnyRequest, _ *PermissionDeps, permUC *usecase.PermissionUsecase) error {
		claims, err := extractClaims(ctx)
		if err != nil {
			return err
		}

		ok, err := permUC.HasSystemPermission(ctx, claims.UserID, permKey)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if !ok {
			return permissionDenied(permKey)
		}

		return nil
	}
}

// ===== 汎用 helper =====

// extractClaims はコンテキストから AuthClaims を取り出し、なければ Unauthenticated を返す.
func extractClaims(ctx context.Context) (*auth.AuthClaims, error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	return claims, nil
}

// permissionDenied wraps an error as PermissionDenied.
func permissionDenied(key string) error {
	return connect.NewError(connect.CodePermissionDenied, errors.Errorf("permission required: %s", key))
}

// requirePerm は 1 件の permission を判定して足りなければ PermissionDenied を返す.
func requirePerm(ctx context.Context, permUC *usecase.PermissionUsecase, userID, groupID, key string) error {
	ok, err := permUC.HasPermission(ctx, userID, groupID, key)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if !ok {
		return permissionDenied(key)
	}

	return nil
}

// ===== 汎用 resolver factory =====

// idExtractor[T] は req.Any() を *T にキャストして string を取り出す.
type idExtractor[T any] func(*T) string

// checkHostPermission は host_id を含む RPC 用. host.group_id に対して permKey を要求する.
func checkHostPermission[T any](permKey string, extract idExtractor[T]) permissionCheck {
	return func(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
		typed, ok := req.Any().(*T)
		if !ok {
			return connect.NewError(connect.CodeInternal, errors.New("unexpected request type in permission rule"))
		}

		claims, err := extractClaims(ctx)
		if err != nil {
			return err
		}

		hostID := strings.TrimSpace(extract(typed))
		if hostID == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("host_id is required"))
		}

		groupID, err := deps.HostRepo.GetGroupID(ctx, hostID)
		if err != nil {
			return convertErr(err)
		}

		return requirePerm(ctx, permUC, claims.UserID, groupID, permKey)
	}
}

func checkSessionPermission[T any](permKey string, extract idExtractor[T]) permissionCheck {
	return func(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
		typed, ok := req.Any().(*T)
		if !ok {
			return connect.NewError(connect.CodeInternal, errors.New("unexpected request type in permission rule"))
		}

		claims, err := extractClaims(ctx)
		if err != nil {
			return err
		}

		sessionID := strings.TrimSpace(extract(typed))
		if sessionID == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("session_id is required"))
		}

		s, err := deps.SessionRepo.Get(ctx, sessionID)
		if err != nil {
			return convertErr(err)
		}

		return requirePerm(ctx, permUC, claims.UserID, s.GroupID, permKey)
	}
}

func checkAccountPermission[T any](permKey string, extract idExtractor[T]) permissionCheck {
	return func(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
		typed, ok := req.Any().(*T)
		if !ok {
			return connect.NewError(connect.CodeInternal, errors.New("unexpected request type in permission rule"))
		}

		claims, err := extractClaims(ctx)
		if err != nil {
			return err
		}

		accountID := strings.TrimSpace(extract(typed))
		if accountID == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("account_id is required"))
		}

		a, err := deps.AccountUC.GetHeadlessAccount(ctx, accountID)
		if err != nil {
			return convertErr(err)
		}

		return requirePerm(ctx, permUC, claims.UserID, a.GroupID, permKey)
	}
}

// checkGroupPermission は group_id を直接指定する RPC 用.
// readOnly=true なら所属しているか or system:group.list があれば許可,
// false なら指定の permKey を要求する.
func checkGroupPermission[T any](permKey string, extract idExtractor[T], readOnly bool) permissionCheck {
	return func(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
		typed, ok := req.Any().(*T)
		if !ok {
			return connect.NewError(connect.CodeInternal, errors.New("unexpected request type in permission rule"))
		}

		claims, err := extractClaims(ctx)
		if err != nil {
			return err
		}

		groupID := strings.TrimSpace(extract(typed))
		if groupID == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("group_id is required"))
		}

		if readOnly {
			// 所属していれば閲覧可. system:group.list でもOK.
			canList, err := permUC.HasSystemPermission(ctx, claims.UserID, entity.PermKey_SystemGroupList)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			if canList {
				return nil
			}

			if _, err := deps.GroupRepo.Get(ctx, groupID); err != nil {
				return convertErr(err)
			}

			memberPerms, err := permUC.HasPermission(ctx, claims.UserID, groupID, entity.PermKey_GroupEdit)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			if memberPerms {
				return nil
			}
			// 所属チェック: GetUserPermissionsForGroup が空配列でないなら所属
			perms, err := deps.GroupRepo.ListByUser(ctx, claims.UserID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			for _, g := range perms {
				if g.ID == groupID {
					return nil
				}
			}

			return permissionDenied(permKey)
		}

		return requirePerm(ctx, permUC, claims.UserID, groupID, permKey)
	}
}

// ===== 個別チェック (resolver / 複合 perm) =====

// checkStartHeadlessHost: StartHeadlessHost は host:write + (account.group_id に対し account:use).
// group_id 未指定なら account.group_id にフォールバック (同一グループ制約).
func checkStartHeadlessHost(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
	msg, ok := req.Any().(*hdlctrlv1.StartHeadlessHostRequest)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("unexpected request type"))
	}

	claims, err := extractClaims(ctx)
	if err != nil {
		return err
	}

	accID := msg.GetHeadlessAccountId()
	if accID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("headless_account_id is required"))
	}

	acc, accErr := deps.AccountUC.GetHeadlessAccount(ctx, accID)
	if accErr != nil {
		// 既存テスト互換: アカウント未存在は Internal (旧実装の挙動).
		// domain.ErrNotFound でラップされていない素の pgx.ErrNoRows を返すため.
		return convertErr(accErr)
	}

	requestedGroupID := msg.GetGroupId()
	if requestedGroupID != "" && acc.GroupID != requestedGroupID {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("account group does not match requested host group"))
	}

	groupID := acc.GroupID

	if err := requirePerm(ctx, permUC, claims.UserID, groupID, entity.PermKey_HostWrite); err != nil {
		return err
	}

	return requirePerm(ctx, permUC, claims.UserID, groupID, entity.PermKey_AccountUse)
}

// checkCreateHeadlessAccount: アカウント作成. group_id 未指定なら personal group.
func checkCreateHeadlessAccount(ctx context.Context, req connect.AnyRequest, _ *PermissionDeps, permUC *usecase.PermissionUsecase) error {
	msg, ok := req.Any().(*hdlctrlv1.CreateHeadlessAccountRequest)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("unexpected request type"))
	}

	claims, err := extractClaims(ctx)
	if err != nil {
		return err
	}

	groupID, err := permUC.ResolveGroupIDForUser(ctx, claims.UserID, msg.GetGroupId())
	if err != nil {
		return convertErr(err)
	}

	return requirePerm(ctx, permUC, claims.UserID, groupID, entity.PermKey_AccountWrite)
}

// checkStartWorld: host:use + account:use + session:write を host.group_id に対して.
// session.group_id == host.group_id == account.group_id (同一グループ制約) を満たすこと.
func checkStartWorld(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
	msg, ok := req.Any().(*hdlctrlv1.StartWorldRequest)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("unexpected request type"))
	}

	claims, err := extractClaims(ctx)
	if err != nil {
		return err
	}

	hostID := msg.GetHostId()
	if hostID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("host_id is required"))
	}

	hostGroupID, err := deps.HostRepo.GetGroupID(ctx, hostID)
	if err != nil {
		return convertErr(err)
	}

	if requested := msg.GetGroupId(); requested != "" && requested != hostGroupID {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("session group must equal host group"))
	}

	for _, key := range []string{
		entity.PermKey_HostUse,
		entity.PermKey_AccountUse,
		entity.PermKey_SessionWrite,
	} {
		if err := requirePerm(ctx, permUC, claims.UserID, hostGroupID, key); err != nil {
			return err
		}
	}

	return nil
}

func checkUpdateSessionParameters(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
	msg, ok := req.Any().(*hdlctrlv1.UpdateSessionParametersRequest)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("unexpected request type"))
	}

	// UpdateSessionParameters の host_id は outer optional, 実体は inner parameters.session_id.
	// session を DB lookup して group_id を得る.
	sessionID := msg.GetParameters().GetSessionId()
	if sessionID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("session_id is required"))
	}

	claims, err := extractClaims(ctx)
	if err != nil {
		return err
	}

	s, err := deps.SessionRepo.Get(ctx, sessionID)
	if err != nil {
		return convertErr(err)
	}

	return requirePerm(ctx, permUC, claims.UserID, s.GroupID, entity.PermKey_SessionWrite)
}

// ===== GroupService / RoleService の個別チェック =====

// checkUpdateGroupMemberRole: personal なら system:group.manage 必須.
func checkUpdateGroupMemberRole(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
	msg, ok := req.Any().(*hdlctrlv1.UpdateGroupMemberRoleRequest)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("unexpected request type"))
	}

	claims, err := extractClaims(ctx)
	if err != nil {
		return err
	}

	groupID := msg.GetGroupId()
	if groupID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("group_id is required"))
	}

	g, err := deps.GroupRepo.Get(ctx, groupID)
	if err != nil {
		return convertErr(err)
	}

	if g.Type == entity.GroupType_Personal {
		// personal は system:group.manage のみ許可.
		ok, err := permUC.HasSystemPermission(ctx, claims.UserID, entity.PermKey_SystemGroupManage)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if !ok {
			return permissionDenied(entity.PermKey_SystemGroupManage)
		}

		return nil
	}

	return requirePerm(ctx, permUC, claims.UserID, groupID, entity.PermKey_GroupMembersManage)
}

// checkCreateRole: group_id 指定なら group:members.manage, 未指定 (グローバル) なら system:role.manage.
func checkCreateRole(ctx context.Context, req connect.AnyRequest, _ *PermissionDeps, permUC *usecase.PermissionUsecase) error {
	msg, ok := req.Any().(*hdlctrlv1.CreateRoleRequest)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("unexpected request type"))
	}

	claims, err := extractClaims(ctx)
	if err != nil {
		return err
	}

	if msg.GroupId == nil || msg.GetGroupId() == "" {
		ok, err := permUC.HasSystemPermission(ctx, claims.UserID, entity.PermKey_SystemRoleManage)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if !ok {
			return permissionDenied(entity.PermKey_SystemRoleManage)
		}

		return nil
	}

	return requirePerm(ctx, permUC, claims.UserID, msg.GetGroupId(), entity.PermKey_GroupMembersManage)
}

// checkUpdateRole / checkDeleteRole: 対象 role が global なら system:role.manage, グループ内なら group:members.manage.
func checkUpdateRole(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
	msg, ok := req.Any().(*hdlctrlv1.UpdateRoleRequest)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("unexpected request type"))
	}

	return checkRoleMutation(ctx, msg.GetRoleId(), deps, permUC)
}

func checkDeleteRole(ctx context.Context, req connect.AnyRequest, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
	msg, ok := req.Any().(*hdlctrlv1.DeleteRoleRequest)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("unexpected request type"))
	}

	return checkRoleMutation(ctx, msg.GetRoleId(), deps, permUC)
}

func checkRoleMutation(ctx context.Context, roleID string, deps *PermissionDeps, permUC *usecase.PermissionUsecase) error {
	if roleID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("role_id is required"))
	}

	claims, err := extractClaims(ctx)
	if err != nil {
		return err
	}

	role, err := deps.RoleRepo.Get(ctx, roleID)
	if err != nil {
		return convertErr(err)
	}

	if role.GroupID == nil || *role.GroupID == "" {
		ok, err := permUC.HasSystemPermission(ctx, claims.UserID, entity.PermKey_SystemRoleManage)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if !ok {
			return permissionDenied(entity.PermKey_SystemRoleManage)
		}

		return nil
	}

	return requirePerm(ctx, permUC, claims.UserID, *role.GroupID, entity.PermKey_GroupMembersManage)
}
