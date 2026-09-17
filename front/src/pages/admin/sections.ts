import { PERMISSION_KEYS } from "../../libs/permissionUtils";

export type AdminSection = {
  titleKey: string;
  href: string;
  /** いずれかを持てばその画面を開ける. */
  permissions: string[];
};

/**
 * システム管理配下の画面一覧.
 * サイドバーのサブ項目と /admin のリダイレクト先の決定に使う.
 */
export const ADMIN_SECTIONS: AdminSection[] = [
  {
    titleKey: "routes.adminUsers",
    href: "/admin/users",
    // ユーザー管理画面は list/create/delete のいずれかで開ける.
    permissions: [
      PERMISSION_KEYS.SYSTEM_USER_LIST,
      PERMISSION_KEYS.SYSTEM_USER_CREATE,
      PERMISSION_KEYS.SYSTEM_USER_DELETE,
    ],
  },
  {
    titleKey: "routes.adminGroups",
    href: "/admin/groups",
    permissions: [PERMISSION_KEYS.SYSTEM_GROUP_LIST],
  },
  {
    titleKey: "routes.adminRoles",
    href: "/admin/roles",
    permissions: [PERMISSION_KEYS.SYSTEM_ROLE_MANAGE],
  },
];
