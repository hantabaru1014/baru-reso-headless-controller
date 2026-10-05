-- name: AddInvitationGroupMember :execrows
-- 未使用の招待にのみ追加できる. FOR SHARE で招待行をロックし、並行する
-- RegisterWithToken (used_at の UPDATE) との間で参加予定の取りこぼしを防ぐ.
INSERT INTO invitation_group_members (invitation_id, group_id, role_id, added_by)
SELECT rt.id, @group_id::text, @role_id::text, sqlc.narg(added_by)::text
FROM registration_tokens rt
WHERE rt.id = @invitation_id::text AND rt.used_at IS NULL
FOR SHARE;

-- name: RemoveInvitationGroupMember :execrows
DELETE FROM invitation_group_members WHERE group_id = $1 AND invitation_id = $2;

-- name: UpdateInvitationGroupMemberRole :execrows
UPDATE invitation_group_members SET role_id = $3 WHERE group_id = $1 AND invitation_id = $2;

-- name: GetInvitationGroupMember :one
SELECT sqlc.embed(invitation_group_members), rt.resonite_id, rt.expires_at
FROM invitation_group_members
JOIN registration_tokens rt ON rt.id = invitation_group_members.invitation_id
WHERE invitation_group_members.group_id = $1
  AND invitation_group_members.invitation_id = $2;

-- name: ListInvitationGroupMembersByGroup :many
SELECT sqlc.embed(invitation_group_members), rt.resonite_id, rt.expires_at
FROM invitation_group_members
JOIN registration_tokens rt ON rt.id = invitation_group_members.invitation_id
WHERE invitation_group_members.group_id = $1
ORDER BY invitation_group_members.added_at;

-- name: MoveInvitationGroupMembersToUser :exec
-- 招待の参加予定を登録ユーザーの group_members へ移す (RegisterWithToken 用).
WITH moved AS (
    DELETE FROM invitation_group_members
    WHERE invitation_id = @invitation_id::text
    RETURNING group_id, role_id, added_by
)
INSERT INTO group_members (group_id, user_id, role_id, added_by)
SELECT group_id, @user_id::text, role_id, added_by FROM moved;
