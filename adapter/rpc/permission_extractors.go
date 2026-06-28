package rpc

import (
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
)

// ===== Host ID extractors =====

func hostIDFromGet(r *hdlctrlv1.GetHeadlessHostRequest) string { return r.GetHostId() }
func hostIDFromGetLogs(r *hdlctrlv1.GetHeadlessHostLogsRequest) string {
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
func hostIDFromKick(r *hdlctrlv1.KickUserRequest) string                 { return r.GetHostId() }
func hostIDFromUpdateUserRole(r *hdlctrlv1.UpdateUserRoleRequest) string { return r.GetHostId() }
func hostIDFromInviteUser(r *hdlctrlv1.InviteUserRequest) string         { return r.GetHostId() }
func hostIDFromListUsersInSession(r *hdlctrlv1.ListUsersInSessionRequest) string {
	return r.GetHostId()
}

// ===== Account ID extractors =====

func accountIDFromDelete(r *hdlctrlv1.DeleteHeadlessAccountRequest) string { return r.GetAccountId() }
func accountIDFromUpdateCreds(r *hdlctrlv1.UpdateHeadlessAccountCredentialsRequest) string {
	return r.GetAccountId()
}
func accountIDFromStorageInfo(r *hdlctrlv1.GetHeadlessAccountStorageInfoRequest) string {
	return r.GetAccountId()
}
func accountIDFromRefetch(r *hdlctrlv1.RefetchHeadlessAccountInfoRequest) string {
	return r.GetAccountId()
}
func accountIDFromUpdateIcon(r *hdlctrlv1.UpdateHeadlessAccountIconRequest) string {
	return r.GetAccountId()
}
func accountIDFromGetFriendRequests(r *hdlctrlv1.GetFriendRequestsRequest) string {
	return r.GetHeadlessAccountId()
}
func accountIDFromAcceptFriends(r *hdlctrlv1.AcceptFriendRequestsRequest) string {
	return r.GetHeadlessAccountId()
}
func accountIDFromListContacts(r *hdlctrlv1.ListContactsRequest) string {
	return r.GetHeadlessAccountId()
}
func accountIDFromGetMessages(r *hdlctrlv1.GetContactMessagesRequest) string {
	return r.GetHeadlessAccountId()
}
func accountIDFromSendMessage(r *hdlctrlv1.SendContactMessageRequest) string {
	return r.GetHeadlessAccountId()
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

func groupIDFromGet(r *hdlctrlv1.GetGroupRequest) string         { return r.GetGroupId() }
func groupIDFromUpdate(r *hdlctrlv1.UpdateGroupRequest) string   { return r.GetGroupId() }
func groupIDFromDelete(r *hdlctrlv1.DeleteGroupRequest) string   { return r.GetGroupId() }
func groupIDFromListMembers(r *hdlctrlv1.ListGroupMembersRequest) string {
	return r.GetGroupId()
}
func groupIDFromAddMember(r *hdlctrlv1.AddGroupMemberRequest) string {
	return r.GetGroupId()
}
func groupIDFromRemoveMember(r *hdlctrlv1.RemoveGroupMemberRequest) string {
	return r.GetGroupId()
}
