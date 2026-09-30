package rpc

import (
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
)

// ===== Host ID extractors =====

func hostIDFromGet(r *hdlctrlv1.GetHeadlessHostRequest) string { return r.GetHostId() }
func hostIDFromGetLogs(r *hdlctrlv1.GetHeadlessHostLogsRequest) string {
	return r.GetHostId()
}

func hostIDFromSearchLogs(r *hdlctrlv1.SearchHeadlessHostLogsRequest) string {
	return r.GetHostId()
}

func hostIDFromListInstances(r *hdlctrlv1.ListHeadlessHostInstancesRequest) string {
	return r.GetHostId()
}
func hostIDFromShutdown(r *hdlctrlv1.ShutdownHeadlessHostRequest) string { return r.GetHostId() }
func hostIDFromKill(r *hdlctrlv1.KillHeadlessHostRequest) string         { return r.GetHostId() }
func hostIDFromUpdateSettings(r *hdlctrlv1.UpdateHeadlessHostSettingsRequest) string {
	return r.GetHostId()
}
func hostIDFromRestart(r *hdlctrlv1.RestartHeadlessHostRequest) string { return r.GetHostId() }
func hostIDFromDelete(r *hdlctrlv1.DeleteHeadlessHostRequest) string   { return r.GetHostId() }
func hostIDFromAllowAccess(r *hdlctrlv1.AllowHostAccessRequest) string { return r.GetHostId() }
func hostIDFromDenyAccess(r *hdlctrlv1.DenyHostAccessRequest) string   { return r.GetHostId() }
func hostIDFromFetchWorldInfo(r *hdlctrlv1.FetchWorldInfoRequest) string {
	return r.GetHostId()
}
func hostIDFromSearchUserInfo(r *hdlctrlv1.SearchUserInfoRequest) string { return r.GetHostId() }
func hostIDFromGetOwnWorlds(r *hdlctrlv1.GetOwnWorldsRequest) string     { return r.GetHostId() }
func hostIDFromBan(r *hdlctrlv1.BanUserRequest) string                   { return r.GetHostId() }
func hostIDFromListBans(r *hdlctrlv1.ListBansRequest) string             { return r.GetHostId() }
func hostIDFromUnban(r *hdlctrlv1.UnbanUserRequest) string               { return r.GetHostId() }
func hostIDFromKick(r *hdlctrlv1.KickUserRequest) string                 { return r.GetHostId() }
func hostIDFromRespawn(r *hdlctrlv1.RespawnUserRequest) string           { return r.GetHostId() }
func hostIDFromSpawnItem(r *hdlctrlv1.SpawnItemRequest) string           { return r.GetHostId() }
func hostIDFromSendDynamicImpulse(r *hdlctrlv1.SendDynamicImpulseRequest) string {
	return r.GetHostId()
}
func hostIDFromUpdateUserRole(r *hdlctrlv1.UpdateUserRoleRequest) string { return r.GetHostId() }
func hostIDFromInviteUser(r *hdlctrlv1.InviteUserRequest) string         { return r.GetHostId() }
func hostIDFromListUsersInSession(r *hdlctrlv1.ListUsersInSessionRequest) string {
	return r.GetHostId()
}

// ===== Account ref extractors =====
//
// アカウントは (group_id, resonite_id) で一意なので、group_id と account_id の組を返す.

func accountRefFromDelete(r *hdlctrlv1.DeleteHeadlessAccountRequest) (string, string) {
	return r.GetGroupId(), r.GetAccountId()
}
func accountRefFromUpdateCreds(r *hdlctrlv1.UpdateHeadlessAccountCredentialsRequest) (string, string) {
	return r.GetGroupId(), r.GetAccountId()
}
func accountRefFromStorageInfo(r *hdlctrlv1.GetHeadlessAccountStorageInfoRequest) (string, string) {
	return r.GetGroupId(), r.GetAccountId()
}
func accountRefFromRefetch(r *hdlctrlv1.RefetchHeadlessAccountInfoRequest) (string, string) {
	return r.GetGroupId(), r.GetAccountId()
}
func accountRefFromUpdateIcon(r *hdlctrlv1.UpdateHeadlessAccountIconRequest) (string, string) {
	return r.GetGroupId(), r.GetAccountId()
}
func accountRefFromGetFriendRequests(r *hdlctrlv1.GetFriendRequestsRequest) (string, string) {
	return r.GetGroupId(), r.GetHeadlessAccountId()
}
func accountRefFromAcceptFriends(r *hdlctrlv1.AcceptFriendRequestsRequest) (string, string) {
	return r.GetGroupId(), r.GetHeadlessAccountId()
}
func accountRefFromSendFriendRequest(r *hdlctrlv1.SendFriendRequestRequest) (string, string) {
	return r.GetGroupId(), r.GetHeadlessAccountId()
}
func accountRefFromRemoveContact(r *hdlctrlv1.RemoveContactRequest) (string, string) {
	return r.GetGroupId(), r.GetHeadlessAccountId()
}
func accountRefFromListContacts(r *hdlctrlv1.ListContactsRequest) (string, string) {
	return r.GetGroupId(), r.GetHeadlessAccountId()
}
func accountRefFromGetMessages(r *hdlctrlv1.GetContactMessagesRequest) (string, string) {
	return r.GetGroupId(), r.GetHeadlessAccountId()
}
func accountRefFromSendMessage(r *hdlctrlv1.SendContactMessageRequest) (string, string) {
	return r.GetGroupId(), r.GetHeadlessAccountId()
}

// ===== Session ID extractors =====

func sessionIDFromGetDetails(r *hdlctrlv1.GetSessionDetailsRequest) string { return r.GetSessionId() }
func sessionIDFromStop(r *hdlctrlv1.StopSessionRequest) string             { return r.GetSessionId() }
func sessionIDFromDelete(r *hdlctrlv1.DeleteEndedSessionRequest) string    { return r.GetSessionId() }
func sessionIDFromSave(r *hdlctrlv1.SaveSessionWorldRequest) string        { return r.GetSessionId() }
func sessionIDFromPrepareDownload(r *hdlctrlv1.PrepareSessionWorldDownloadRequest) string {
	return r.GetSessionId()
}
func sessionIDFromUpdateExtra(r *hdlctrlv1.UpdateSessionExtraSettingsRequest) string {
	return r.GetSessionId()
}
func sessionIDFromIssueLink(r *hdlctrlv1.IssueResoniteLinkConnectionRequest) string {
	return r.GetSessionId()
}

// ===== Group ID extractors =====

func groupIDFromGet(r *hdlctrlv1.GetGroupRequest) string       { return r.GetGroupId() }
func groupIDFromUpdate(r *hdlctrlv1.UpdateGroupRequest) string { return r.GetGroupId() }
func groupIDFromDelete(r *hdlctrlv1.DeleteGroupRequest) string { return r.GetGroupId() }
func groupIDFromListMembers(r *hdlctrlv1.ListGroupMembersRequest) string {
	return r.GetGroupId()
}
func groupIDFromAddMember(r *hdlctrlv1.AddGroupMemberRequest) string {
	return r.GetGroupId()
}
func groupIDFromRemoveMember(r *hdlctrlv1.RemoveGroupMemberRequest) string {
	return r.GetGroupId()
}
