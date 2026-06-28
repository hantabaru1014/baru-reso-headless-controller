import { GroupType, RoleScope } from "../../pbgen/hdlctrl/v1/permission_pb";

/**
 * 既知のパーミッションキー一覧.
 * docs/permissions.md の "4. パーミッション一覧" と対応する.
 */
export const PERMISSION_KEYS = {
  HOST_READ: "host:read",
  HOST_WRITE: "host:write",
  HOST_USE: "host:use",
  SESSION_READ: "session:read",
  SESSION_WRITE: "session:write",
  ACCOUNT_READ: "account:read",
  ACCOUNT_WRITE: "account:write",
  ACCOUNT_USE: "account:use",
  GROUP_MEMBERS_MANAGE: "group:members.manage",
  GROUP_EDIT: "group:edit",
  SYSTEM_USER_CREATE: "system:user.create",
  SYSTEM_USER_DELETE: "system:user.delete",
  SYSTEM_USER_LIST: "system:user.list",
  SYSTEM_GROUP_LIST: "system:group.list",
  SYSTEM_GROUP_MANAGE: "system:group.manage",
  SYSTEM_ROLE_MANAGE: "system:role.manage",
} as const;

export type PermissionKey =
  (typeof PERMISSION_KEYS)[keyof typeof PERMISSION_KEYS];

export function groupTypeToLabel(type: GroupType): string {
  switch (type) {
    case GroupType.PERSONAL:
      return "個人";
    case GroupType.NORMAL:
      return "共有";
    case GroupType.SYSTEM:
      return "システム";
    default:
      return "不明";
  }
}

export function roleScopeToLabel(scope: RoleScope): string {
  switch (scope) {
    case RoleScope.NORMAL:
      return "通常";
    case RoleScope.SYSTEM:
      return "システム";
    default:
      return "不明";
  }
}

/**
 * UI 表示用に permission_key を人間が読める日本語に変換する.
 * 未知のキーはそのまま返す.
 */
export function permissionKeyToLabel(key: string): string {
  switch (key) {
    case PERMISSION_KEYS.HOST_READ:
      return "ホスト閲覧";
    case PERMISSION_KEYS.HOST_WRITE:
      return "ホスト管理";
    case PERMISSION_KEYS.HOST_USE:
      return "ホスト利用 (セッション開始)";
    case PERMISSION_KEYS.SESSION_READ:
      return "セッション閲覧";
    case PERMISSION_KEYS.SESSION_WRITE:
      return "セッション管理";
    case PERMISSION_KEYS.ACCOUNT_READ:
      return "アカウント閲覧";
    case PERMISSION_KEYS.ACCOUNT_WRITE:
      return "アカウント管理";
    case PERMISSION_KEYS.ACCOUNT_USE:
      return "アカウント利用 (セッション開始)";
    case PERMISSION_KEYS.GROUP_MEMBERS_MANAGE:
      return "グループメンバー管理";
    case PERMISSION_KEYS.GROUP_EDIT:
      return "グループ編集";
    case PERMISSION_KEYS.SYSTEM_USER_CREATE:
      return "ユーザー作成";
    case PERMISSION_KEYS.SYSTEM_USER_DELETE:
      return "ユーザー削除";
    case PERMISSION_KEYS.SYSTEM_USER_LIST:
      return "全ユーザー閲覧";
    case PERMISSION_KEYS.SYSTEM_GROUP_LIST:
      return "全グループ閲覧";
    case PERMISSION_KEYS.SYSTEM_GROUP_MANAGE:
      return "全グループ管理";
    case PERMISSION_KEYS.SYSTEM_ROLE_MANAGE:
      return "グローバルロール管理";
    default:
      return key;
  }
}
