package rpc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestControllerService_ListHeadlessAccounts(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.Cleanup()

	client := setupAuthenticatedClient(t, setup.service)

	// Create 25 test headless accounts for pagination
	const totalAccounts = 25
	for i := 1; i <= totalAccounts; i++ {
		id := fmt.Sprintf("U-test%02d", i)
		testutil.CreateTestHeadlessAccount(t, setup.queries, id, id+"@example.test", "password")
	}

	t.Run("成功: ヘッドレスアカウントのリストを取得 (ページング検証 / system:group.list 保持者は全件)", func(t *testing.T) {
		// page 未指定 -> デフォルト 20 件
		// 既定ユーザーは system-admin (system:group.list 保持) なので
		// group_id 未指定でも全グループのアカウントを返す.
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessAccountsRequest{})

		res, err := client.ListHeadlessAccounts(t.Context(), req)
		require.NoError(t, err)

		assert.Len(t, res.Msg.GetAccounts(), 20)
		require.NotNil(t, res.Msg.GetPage())
		assert.Equal(t, int32(totalAccounts), res.Msg.GetPage().GetTotalCount())
		assert.Equal(t, int32(0), res.Msg.GetPage().GetPageIndex())
		assert.Equal(t, int32(20), res.Msg.GetPage().GetPageSize())

		// Verify account data
		account1 := res.Msg.GetAccounts()[0]
		assert.NotEmpty(t, account1.GetUserId())
		assert.NotEmpty(t, account1.GetUserName())

		// page_index=1, page_size=20 -> 残り 5 件
		req2 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessAccountsRequest{
			Page: &hdlctrlv1.PageRequest{PageIndex: 1, PageSize: 20},
		})
		res2, err := client.ListHeadlessAccounts(t.Context(), req2)
		require.NoError(t, err)
		assert.Len(t, res2.Msg.GetAccounts(), 5)
		assert.Equal(t, int32(totalAccounts), res2.Msg.GetPage().GetTotalCount())

		// page_size=50 -> 全件
		req3 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessAccountsRequest{
			Page: &hdlctrlv1.PageRequest{PageIndex: 0, PageSize: 50},
		})
		res3, err := client.ListHeadlessAccounts(t.Context(), req3)
		require.NoError(t, err)
		assert.Len(t, res3.Msg.GetAccounts(), totalAccounts)

		// page_size=150 -> 100 にクランプ。25件しかないので全件返るが PageSize は 100 として返る
		req4 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessAccountsRequest{
			Page: &hdlctrlv1.PageRequest{PageIndex: 0, PageSize: 150},
		})
		res4, err := client.ListHeadlessAccounts(t.Context(), req4)
		require.NoError(t, err)
		assert.Len(t, res4.Msg.GetAccounts(), totalAccounts)
		assert.Equal(t, int32(100), res4.Msg.GetPage().GetPageSize())
	})

	t.Run("グループフィルタ: 指定 / 未指定 / 権限なしの分岐", func(t *testing.T) {
		// 別 setup で fresh DB を使う (totalAccounts と混じらないように)
		setup2 := setupControllerServiceTest(t)
		defer setup2.Cleanup()

		client2 := setupAuthenticatedClient(t, setup2.service)

		const otherUserID = "U-other-acc"

		personalGID := testutil.SetupNormalUserWithPersonalGroup(t, setup2.queries, otherUserID)

		sharedGID := "group-shared-acc"
		testutil.CreateTestGroup(t, setup2.queries, sharedGID, otherUserID)

		testutil.CreateTestHeadlessAccount(t, setup2.queries, "U-migrated1", "m1@example.test", "p")
		testutil.CreateTestHeadlessAccount(t, setup2.queries, "U-migrated2", "m2@example.test", "p")
		testutil.CreateTestHeadlessAccountInGroup(t, setup2.queries, "U-personal", "p@example.test", "p", personalGID)
		testutil.CreateTestHeadlessAccountInGroup(t, setup2.queries, "U-shared", "s@example.test", "p", sharedGID)

		// case 1: non-admin / group_id 未指定 -> personal + shared の 2 件のみ.
		reqAuto := testutil.CreateAuthenticatedRequest(
			t, &hdlctrlv1.ListHeadlessAccountsRequest{},
			otherUserID, "U-other-resonite", "https://example.test/icon.png",
		)
		resAuto, err := client2.ListHeadlessAccounts(t.Context(), reqAuto)
		require.NoError(t, err)
		assert.Equal(t, int32(2), resAuto.Msg.GetPage().GetTotalCount(),
			"non-admin should only see accounts in groups they belong to")

		// case 2: non-admin / 自分が所属する group_id を指定 -> その group のみ.
		reqExplicit := testutil.CreateAuthenticatedRequest(
			t, &hdlctrlv1.ListHeadlessAccountsRequest{GroupId: &sharedGID},
			otherUserID, "U-other-resonite", "https://example.test/icon.png",
		)
		resExplicit, err := client2.ListHeadlessAccounts(t.Context(), reqExplicit)
		require.NoError(t, err)
		assert.Equal(t, int32(1), resExplicit.Msg.GetPage().GetTotalCount())

		// case 3: non-admin / 権限の無い group_id 指定 -> PermissionDenied.
		forbiddenGID := entity.MigratedPrePermissionGroupID

		reqForbidden := testutil.CreateAuthenticatedRequest(
			t, &hdlctrlv1.ListHeadlessAccountsRequest{GroupId: &forbiddenGID},
			otherUserID, "U-other-resonite", "https://example.test/icon.png",
		)
		_, err = client2.ListHeadlessAccounts(t.Context(), reqForbidden)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())

		// case 4: 既定ユーザー (system-admin) が同じ group_id を指定 -> system:group.list 経由で許可.
		reqAdmin := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessAccountsRequest{
			GroupId: &sharedGID,
		})
		resAdmin, err := client2.ListHeadlessAccounts(t.Context(), reqAdmin)
		require.NoError(t, err)
		assert.Equal(t, int32(1), resAdmin.Msg.GetPage().GetTotalCount())
	})
}

func TestControllerService_CreateHeadlessAccount(t *testing.T) {
	t.Run("成功: 有効な認証情報でアカウント作成", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Mock skyfrost client to return successful login
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "testuser@example.test", "testpass123").
			Return(&skyfrost.UserSession{UserId: "U-testuser123"}, nil)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-testuser123").
			Return(&skyfrost.UserInfo{
				ID:       "U-testuser123",
				UserName: "TestUser",
				IconUrl:  "https://example.test/icon.png",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "testuser@example.test",
			Password:   "testpass123",
		})

		res, err := client.CreateHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify account was created in DB
		account := testutil.GetOnlyHeadlessAccount(t, setup.queries, "U-testuser123")
		assert.Equal(t, "testuser@example.test", account.Credential)
	})

	t.Run("成功: 最小権限 caller (account:write) で group_id 指定で作成", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "mp@example.test", "p").
			Return(&skyfrost.UserSession{UserId: "U-mp-newacc"}, nil)
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-mp-newacc").
			Return(&skyfrost.UserInfo{ID: "U-mp-newacc", UserName: "mp"}, nil)

		const groupID = "g-mp-createacc"
		// caller is added to groupID with exactly account:write
		gid := groupID
		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "mp@example.test",
			Password:   "p",
			GroupId:    &gid,
		}, "U-mp-createacc", groupID, []string{entity.PermKey_AccountWrite})

		_, err := client.CreateHeadlessAccount(t.Context(), req)
		require.NoError(t, err)

		// 作成された account は指定 group_id に所属している.
		acc := testutil.GetOnlyHeadlessAccount(t, setup.queries, "U-mp-newacc")
		assert.Equal(t, groupID, acc.GroupID)
	})

	t.Run("成功: アイコンを指定するとアイコンを設定してから追加する", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "icon@example.test", "p").
			Return(&skyfrost.UserSession{UserId: "U-icon"}, nil)
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-icon").
			Return(&skyfrost.UserInfo{ID: "U-icon", UserName: "IconUser"}, nil)
		setup.mockSkyfrost.EXPECT().
			UploadTextureRecord(gomock.Any(), "icon@example.test", "p", "Profile Icon", "Inventory", gomock.Any()).
			Return("R-icon", "resdb:///icon.webp", nil)
		setup.mockSkyfrost.EXPECT().
			UpdateUserProfile(gomock.Any(), "icon@example.test", "p", &skyfrost.UserProfile{IconUrl: "resdb:///icon.webp"}).
			Return(nil)

		var iconData bytes.Buffer
		require.NoError(t, png.Encode(&iconData, image.NewRGBA(image.Rect(0, 0, 8, 8))))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateHeadlessAccountRequest{ //nolint:gosec // G101: テスト用のダミー認証情報
			Credential: "icon@example.test",
			Password:   "p",
			IconData:   iconData.Bytes(),
		})

		_, err := client.CreateHeadlessAccount(t.Context(), req)
		require.NoError(t, err)

		acc := testutil.GetOnlyHeadlessAccount(t, setup.queries, "U-icon")
		assert.Equal(t, "resdb:///icon.webp", acc.LastIconUrl.String)
	})

	t.Run("失敗: 無効な認証情報でアカウント作成", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Mock skyfrost client to return login error
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "invalid@example.test", "invalidpassword").
			Return(nil, connect.NewError(connect.CodeUnauthenticated, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "invalid@example.test",
			Password:   "invalidpassword",
		})

		_, err := client.CreateHeadlessAccount(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: 同一グループに既に存在するアカウントを作成 (別グループには登録できる)", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create initial account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-existing", "existing@example.test", "password123")

		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "existing@example.test", "newpassword123").
			Return(&skyfrost.UserSession{UserId: "U-existing"}, nil).
			Times(2)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-existing").
			Return(&skyfrost.UserInfo{
				ID:       "U-existing",
				UserName: "ExistingUser",
				IconUrl:  "https://example.test/icon.png",
			}, nil).
			Times(2)

		// アカウントは (group_id, resonite_id) で一意なので、別グループ (caller の
		// personal グループ) には同じ Resonite アカウントを登録できる.
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "existing@example.test",
			Password:   "newpassword123",
		})

		_, err := client.CreateHeadlessAccount(t.Context(), req)
		require.NoError(t, err)

		registered, err := setup.queries.ListHeadlessAccountsByResoniteID(t.Context(), "U-existing")
		require.NoError(t, err)
		require.Len(t, registered, 2)

		// 同一グループへの重複登録は AlreadyExists.
		gid := entity.MigratedPrePermissionGroupID
		reqDup := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateHeadlessAccountRequest{
			Credential: "existing@example.test",
			Password:   "newpassword123",
			GroupId:    &gid,
		})

		_, err = client.CreateHeadlessAccount(t.Context(), reqDup)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())
	})
}

func TestControllerService_RegisterHeadlessAccount(t *testing.T) {
	t.Run("成功: Resonite アカウントを登録する (メール認証前なのでヘッドレスアカウントには追加しない)", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		setup.mockSkyfrost.EXPECT().
			RegisterUser(gomock.Any(), "NewUser", "newuser@example.test", "Passw0rd", time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)).
			Return("U-newuser", nil)

		const groupID = "g-mp-registeracc"

		gid := groupID
		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.RegisterHeadlessAccountRequest{
			Username:    " NewUser ",
			Email:       "NewUser@Example.test",
			Password:    "Passw0rd",
			DateOfBirth: "2000-01-02",
			GroupId:     &gid,
		}, "U-mp-registeracc", groupID, []string{entity.PermKey_AccountWrite})

		res, err := client.RegisterHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, "U-newuser", res.Msg.GetAccountId())

		registered, err := setup.queries.ListHeadlessAccountsByResoniteID(t.Context(), "U-newuser")
		require.NoError(t, err)
		assert.Empty(t, registered)
	})

	t.Run("失敗: 入力が Resonite の登録条件を満たさない場合は InvalidArgument (Resonite に送らない)", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		tooYoung := time.Now().AddDate(-15, 0, 0).Format(time.DateOnly)

		cases := map[string]*hdlctrlv1.RegisterHeadlessAccountRequest{
			"ユーザー名なし":      {Username: " ", Email: "a@example.test", Password: "Passw0rd", DateOfBirth: "2000-01-01"},
			"メールアドレス不正":    {Username: "a", Email: "invalid", Password: "Passw0rd", DateOfBirth: "2000-01-01"},
			"表示名付きメールアドレス": {Username: "a", Email: "A <a@example.test>", Password: "Passw0rd", DateOfBirth: "2000-01-01"},
			"150 歳超":       {Username: "a", Email: "a@example.test", Password: "Passw0rd", DateOfBirth: "1800-01-01"},
			"パスワードが短い":     {Username: "a", Email: "a@example.test", Password: "Pass0rd", DateOfBirth: "2000-01-01"},
			"パスワードに大文字がない": {Username: "a", Email: "a@example.test", Password: "passw0rd", DateOfBirth: "2000-01-01"},
			"パスワードに数字がない":  {Username: "a", Email: "a@example.test", Password: "Password", DateOfBirth: "2000-01-01"},
			"生年月日の形式不正":    {Username: "a", Email: "a@example.test", Password: "Passw0rd", DateOfBirth: "2000/01/01"},
			"16 歳未満":       {Username: "a", Email: "a@example.test", Password: "Passw0rd", DateOfBirth: tooYoung},
		}
		for name, msg := range cases {
			_, err := client.RegisterHeadlessAccount(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, msg))
			require.Error(t, err, name)
			assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
		}
	})

	t.Run("失敗: Resonite 側の登録エラーではアカウントを追加しない (4xx は InvalidArgument)", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		setup.mockSkyfrost.EXPECT().
			RegisterUser(gomock.Any(), "Taken", "taken@example.test", "Passw0rd", gomock.Any()).
			Return("", fmt.Errorf("failed to register: %w", &skyfrost.APIError{StatusCode: 409, Body: "Username is already taken"}))
		setup.mockSkyfrost.EXPECT().
			RegisterUser(gomock.Any(), "Unavailable", "unavailable@example.test", "Passw0rd", gomock.Any()).
			Return("", &skyfrost.APIError{StatusCode: 503, Body: "Service Unavailable"})
		setup.mockSkyfrost.EXPECT().
			RegisterUser(gomock.Any(), "Pending", "pending@example.test", "Passw0rd", gomock.Any()).
			Return("", fmt.Errorf("%w (U-pending): timed out", skyfrost.ErrRegistrationNotConfirmed))

		register := func(username string) error {
			_, err := client.RegisterHeadlessAccount(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RegisterHeadlessAccountRequest{
				Username:    username,
				Email:       strings.ToLower(username) + "@example.test",
				Password:    "Passw0rd",
				DateOfBirth: "2000-01-01",
			}))

			return err
		}

		err := register("Taken")
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		assert.Contains(t, err.Error(), "Username is already taken")

		err = register("Unavailable")
		require.Error(t, err)
		assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))

		// 受け付け後に完了を確認できなかった場合は、既存アカウントとして追加するよう案内する.
		err = register("Pending")
		require.Error(t, err)
		assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assert.Contains(t, err.Error(), "add it as an existing account")

		accounts, err := setup.queries.ListHeadlessAccounts(t.Context())
		require.NoError(t, err)
		assert.Empty(t, accounts)
	})
}

func TestControllerService_DeleteHeadlessAccount(t *testing.T) {
	t.Run("成功: ヘッドレスアカウントを削除", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-todelete", "todelete@example.test", "password123")

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessAccountRequest{
			AccountId: "U-todelete",
		})

		res, err := client.DeleteHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify account was deleted
		remaining, err := setup.queries.ListHeadlessAccountsByResoniteID(t.Context(), "U-todelete")
		require.NoError(t, err)
		assert.Empty(t, remaining)

		// 同一アカウントが複数グループに登録されている場合、group_id 無しでは対象を
		// 特定できず InvalidArgument. group_id を指定するとそのグループの登録だけが消える.
		const otherGroupID = "g-del-other"

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-multi", "multi@example.test", "password123")
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-multi", "multi@example.test", "password123", otherGroupID)

		_, err = client.DeleteHeadlessAccount(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessAccountRequest{
			AccountId: "U-multi",
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

		_, err = client.DeleteHeadlessAccount(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessAccountRequest{
			AccountId: "U-multi",
			GroupId:   otherGroupID,
		}))
		require.NoError(t, err)

		survivor := testutil.GetOnlyHeadlessAccount(t, setup.queries, "U-multi")
		assert.Equal(t, entity.MigratedPrePermissionGroupID, survivor.GroupID)
	})

	t.Run("成功: 最小権限 caller (account:write) で削除", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-delacc"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-target", "x@example.test", "p", groupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.DeleteHeadlessAccountRequest{
			AccountId: "U-mp-target",
		}, "U-mp-delacc", groupID, []string{entity.PermKey_AccountWrite})

		_, err := client.DeleteHeadlessAccount(t.Context(), req)
		require.NoError(t, err)
	})

	// 権限システム導入後は permission interceptor が account の所属グループを
	// 確認するため、存在しないアカウントは NotFound を返す.
	t.Run("失敗: 存在しないアカウントは NotFound", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessAccountRequest{
			AccountId: "U-nonexistent",
		})

		_, err := client.DeleteHeadlessAccount(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_UpdateHeadlessAccountCredentials(t *testing.T) {
	t.Run("成功: 認証情報を更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-update", "old@example.test", "oldpassword")

		// Mock skyfrost client to return successful login with new credentials
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "new@example.test", "newpassword").
			Return(&skyfrost.UserSession{UserId: "U-update"}, nil)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-update").
			Return(&skyfrost.UserInfo{
				ID:       "U-update",
				UserName: "UpdatedUser",
				IconUrl:  "https://example.test/icon.png",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessAccountCredentialsRequest{
			AccountId:  "U-update",
			Credential: "new@example.test",
			Password:   "newpassword",
		})

		res, err := client.UpdateHeadlessAccountCredentials(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify credentials were updated in DB
		account := testutil.GetOnlyHeadlessAccount(t, setup.queries, "U-update")
		assert.Equal(t, "new@example.test", account.Credential)
	})

	t.Run("成功: 最小権限 caller (account:write) で更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-updcred"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-cred", "old@example.test", "old", groupID)

		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "new@example.test", "new").
			Return(&skyfrost.UserSession{UserId: "U-mp-cred"}, nil)
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-mp-cred").
			Return(&skyfrost.UserInfo{ID: "U-mp-cred", UserName: "mp"}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.UpdateHeadlessAccountCredentialsRequest{
			AccountId:  "U-mp-cred",
			Credential: "new@example.test",
			Password:   "new",
		}, "U-mp-updcred", groupID, []string{entity.PermKey_AccountWrite})

		_, err := client.UpdateHeadlessAccountCredentials(t.Context(), req)
		require.NoError(t, err)
	})

	// 権限システム導入後は permission interceptor が account の所属グループを
	// 確認するため、存在しないアカウントは NotFound を返す.
	t.Run("失敗: 存在しないアカウントは NotFound", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessAccountCredentialsRequest{
			AccountId:  "U-nonexist",
			Credential: "nonexist@example.test",
			Password:   "password",
		})

		_, err := client.UpdateHeadlessAccountCredentials(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("失敗: 無効な新しい認証情報", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-invalidupdate", "valid@example.test", "validpassword")

		// Mock skyfrost to return login error
		setup.mockSkyfrost.EXPECT().
			UserLogin(gomock.Any(), "invalid@example.test", "invalidpassword").
			Return(nil, connect.NewError(connect.CodeUnauthenticated, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessAccountCredentialsRequest{
			AccountId:  "U-invalidupdate",
			Credential: "invalid@example.test",
			Password:   "invalidpassword",
		})

		_, err := client.UpdateHeadlessAccountCredentials(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_GetHeadlessAccountStorageInfo(t *testing.T) {
	t.Run("成功: ストレージ情報を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-storage-success", "user@example.test", "password")

		// Mock skyfrost to return storage info
		setup.mockSkyfrost.EXPECT().
			GetStorageInfo(gomock.Any(), "user@example.test", "password", "U-storage-success").
			Return(&skyfrost.StorageInfo{
				UsedBytes:  1024 * 1024 * 100, // 100 MB
				QuotaBytes: 1024 * 1024 * 500, // 500 MB
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessAccountStorageInfoRequest{
			AccountId: "U-storage-success",
		})

		res, err := client.GetHeadlessAccountStorageInfo(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Equal(t, int64(1024*1024*100), res.Msg.GetStorageUsedBytes())
		assert.Equal(t, int64(1024*1024*500), res.Msg.GetStorageQuotaBytes())
	})

	t.Run("成功: 最小権限 caller (account:read) で取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-storage"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-stor", "stor@example.test", "p", groupID)

		setup.mockSkyfrost.EXPECT().
			GetStorageInfo(gomock.Any(), "stor@example.test", "p", "U-mp-stor").
			Return(&skyfrost.StorageInfo{UsedBytes: 1, QuotaBytes: 2}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.GetHeadlessAccountStorageInfoRequest{
			AccountId: "U-mp-stor",
		}, "U-mp-storage", groupID, []string{entity.PermKey_AccountRead})

		_, err := client.GetHeadlessAccountStorageInfo(t.Context(), req)
		require.NoError(t, err)
	})

	// 権限システム導入後は permission interceptor が account の所属グループを
	// 確認するため、存在しないアカウントは NotFound を返す.
	t.Run("失敗: 存在しないアカウントは NotFound", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessAccountStorageInfoRequest{
			AccountId: "U-nonexist",
		})

		_, err := client.GetHeadlessAccountStorageInfo(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("失敗: ストレージ情報の取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-storage", "user@example.test", "password")

		// Mock skyfrost to return error
		setup.mockSkyfrost.EXPECT().
			GetStorageInfo(gomock.Any(), "user@example.test", "password", "U-storage").
			Return(nil, connect.NewError(connect.CodeUnauthenticated, nil))
		// Resonite にログインを拒否された (メール未認証等) 場合.
		setup.mockSkyfrost.EXPECT().
			GetStorageInfo(gomock.Any(), "user@example.test", "password", "U-storage").
			Return(nil, fmt.Errorf("failed to login: %w", &skyfrost.APIError{StatusCode: 403, Body: `"Login.EmailNotVerified"`}))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessAccountStorageInfoRequest{
			AccountId: "U-storage",
		})

		_, err := client.GetHeadlessAccountStorageInfo(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())

		// フロントがリトライしないよう Internal と区別する.
		_, err = client.GetHeadlessAccountStorageInfo(t.Context(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	})
}

func TestControllerService_RefetchHeadlessAccountInfo(t *testing.T) {
	t.Run("成功: アカウント情報を再取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-refetch", "user@example.test", "password")

		// Mock skyfrost to return user info
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-refetch").
			Return(&skyfrost.UserInfo{
				ID:       "U-refetch",
				UserName: "UpdatedName",
				IconUrl:  "https://example.test/new-icon.png",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RefetchHeadlessAccountInfoRequest{
			AccountId: "U-refetch",
		})

		res, err := client.RefetchHeadlessAccountInfo(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify account info was updated
		account := testutil.GetOnlyHeadlessAccount(t, setup.queries, "U-refetch")
		assert.Equal(t, "UpdatedName", account.LastDisplayName.String)
		assert.Equal(t, "https://example.test/new-icon.png", account.LastIconUrl.String)
	})

	t.Run("成功: 最小権限 caller (account:write) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-refetch"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-ref", "r@example.test", "p", groupID)

		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-mp-ref").
			Return(&skyfrost.UserInfo{ID: "U-mp-ref", UserName: "New"}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.RefetchHeadlessAccountInfoRequest{
			AccountId: "U-mp-ref",
		}, "U-mp-refetch", groupID, []string{entity.PermKey_AccountWrite})

		_, err := client.RefetchHeadlessAccountInfo(t.Context(), req)
		require.NoError(t, err)
	})

	// 権限システム導入後は permission interceptor が account の所属グループを
	// 確認するため、存在しないアカウントは NotFound を返す.
	t.Run("失敗: 存在しないアカウントは NotFound", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RefetchHeadlessAccountInfoRequest{
			AccountId: "U-nonexist",
		})

		_, err := client.RefetchHeadlessAccountInfo(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("失敗: ユーザー情報の取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-failfetch", "user@example.test", "password")

		// Mock skyfrost to return error
		setup.mockSkyfrost.EXPECT().
			FetchUserInfo(gomock.Any(), "U-failfetch").
			Return(nil, connect.NewError(connect.CodeNotFound, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RefetchHeadlessAccountInfoRequest{
			AccountId: "U-failfetch",
		})

		_, err := client.RefetchHeadlessAccountInfo(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}
