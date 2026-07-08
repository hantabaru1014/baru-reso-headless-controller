import { GroupType, RoleScope } from "../../pbgen/hdlctrl/v1/permission_pb";
import i18n from "@/libs/i18n";

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
  SYSTEM_MESSAGE_MANAGE: "system:message.manage",
} as const;

export type PermissionKey =
  (typeof PERMISSION_KEYS)[keyof typeof PERMISSION_KEYS];

export function groupTypeToLabel(type: GroupType): string {
  switch (type) {
    case GroupType.PERSONAL:
      return i18n.t("permissionUtils.groupType.personal");
    case GroupType.NORMAL:
      return i18n.t("permissionUtils.groupType.normal");
    case GroupType.SYSTEM:
      return i18n.t("permissionUtils.groupType.system");
    default:
      return i18n.t("common.unknown");
  }
}

export function roleScopeToLabel(scope: RoleScope): string {
  switch (scope) {
    case RoleScope.NORMAL:
      return i18n.t("permissionUtils.roleScope.normal");
    case RoleScope.SYSTEM:
      return i18n.t("permissionUtils.roleScope.system");
    default:
      return i18n.t("common.unknown");
  }
}

/**
 * UI 表示用に permission_key を人間が読める日本語に変換する.
 * 未知のキーはそのまま返す.
 */
export function permissionKeyToLabel(key: string): string {
  switch (key) {
    case PERMISSION_KEYS.HOST_READ:
      return i18n.t("permissionUtils.permissionKey.hostRead");
    case PERMISSION_KEYS.HOST_WRITE:
      return i18n.t("permissionUtils.permissionKey.hostWrite");
    case PERMISSION_KEYS.HOST_USE:
      return i18n.t("permissionUtils.permissionKey.hostUse");
    case PERMISSION_KEYS.SESSION_READ:
      return i18n.t("permissionUtils.permissionKey.sessionRead");
    case PERMISSION_KEYS.SESSION_WRITE:
      return i18n.t("permissionUtils.permissionKey.sessionWrite");
    case PERMISSION_KEYS.ACCOUNT_READ:
      return i18n.t("permissionUtils.permissionKey.accountRead");
    case PERMISSION_KEYS.ACCOUNT_WRITE:
      return i18n.t("permissionUtils.permissionKey.accountWrite");
    case PERMISSION_KEYS.ACCOUNT_USE:
      return i18n.t("permissionUtils.permissionKey.accountUse");
    case PERMISSION_KEYS.GROUP_MEMBERS_MANAGE:
      return i18n.t("permissionUtils.permissionKey.groupMembersManage");
    case PERMISSION_KEYS.GROUP_EDIT:
      return i18n.t("permissionUtils.permissionKey.groupEdit");
    case PERMISSION_KEYS.SYSTEM_USER_CREATE:
      return i18n.t("permissionUtils.permissionKey.systemUserCreate");
    case PERMISSION_KEYS.SYSTEM_USER_DELETE:
      return i18n.t("permissionUtils.permissionKey.systemUserDelete");
    case PERMISSION_KEYS.SYSTEM_USER_LIST:
      return i18n.t("permissionUtils.permissionKey.systemUserList");
    case PERMISSION_KEYS.SYSTEM_GROUP_LIST:
      return i18n.t("permissionUtils.permissionKey.systemGroupList");
    case PERMISSION_KEYS.SYSTEM_GROUP_MANAGE:
      return i18n.t("permissionUtils.permissionKey.systemGroupManage");
    case PERMISSION_KEYS.SYSTEM_ROLE_MANAGE:
      return i18n.t("permissionUtils.permissionKey.systemRoleManage");
    case PERMISSION_KEYS.SYSTEM_MESSAGE_MANAGE:
      return i18n.t("permissionUtils.permissionKey.systemMessageManage");
    default:
      return key;
  }
}
