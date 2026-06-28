import { useQuery } from "@connectrpc/connect-query";
import { useCallback, useMemo } from "react";
import { getMyPermissions } from "../../pbgen/hdlctrl/v1/permission-RoleService_connectquery";
import {
  GroupPermissions,
  MyPermissions,
} from "../../pbgen/hdlctrl/v1/permission_pb";
import { PERMISSION_KEYS, PermissionKey } from "../libs/permissionUtils";

export interface UsePermissionsResult {
  permissions: MyPermissions | undefined;
  isPending: boolean;
  refetch: () => void;
  /**
   * 特定グループに対するパーミッションを持つか.
   * system:group.manage を持つユーザーは normal/personal グループ権限を暗黙に持つ扱い.
   * (backend で展開済みの場合もあるが、念のためフロント側でもチェックする)
   */
  hasPermission: (
    groupId: string | undefined | null,
    key: PermissionKey | string,
  ) => boolean;
  /**
   * グループ非依存のシステムパーミッションを持つか.
   */
  hasSystemPermission: (key: PermissionKey | string) => boolean;
  /**
   * 指定 key を持つグループ ID 一覧 (リソース作成時の選択肢などに利用).
   */
  groupsWithPermission: (key: PermissionKey | string) => GroupPermissions[];
}

/**
 * ログインユーザーの権限情報を取得し、判定ヘルパーを提供する.
 *
 * 例:
 * ```tsx
 * const { hasPermission, hasSystemPermission } = usePermissions();
 * const canEdit = hasPermission(host.groupId, PERMISSION_KEYS.HOST_WRITE);
 * if (hasSystemPermission(PERMISSION_KEYS.SYSTEM_ROLE_MANAGE)) { ... }
 * ```
 */
export function usePermissions(): UsePermissionsResult {
  const { data, isPending, refetch } = useQuery(getMyPermissions, undefined, {
    // 権限は頻繁に変わらないので staleTime を長めに
    staleTime: 5 * 60 * 1000,
  });
  const permissions = data?.permissions;

  const hasSystemPermission = useCallback(
    (key: PermissionKey | string) => {
      return permissions?.systemPermissionKeys.includes(key) ?? false;
    },
    [permissions],
  );

  const hasPermission = useCallback(
    (groupId: string | undefined | null, key: PermissionKey | string) => {
      // system:group.manage を持つ場合は normal/personal グループ操作を暗黙に許可.
      // groupId が空でもこの bypass は適用する (空 groupId のリソースに対する
      // system-admin の操作を UI 側で塞がないため).
      // system scope のパーミッションには適用しない.
      if (
        !key.startsWith("system:") &&
        hasSystemPermission(PERMISSION_KEYS.SYSTEM_GROUP_MANAGE)
      ) {
        return true;
      }
      if (!groupId) return false;
      const group = permissions?.groups.find((g) => g.groupId === groupId);
      return group?.permissionKeys.includes(key) ?? false;
    },
    [permissions, hasSystemPermission],
  );

  const groupsWithPermission = useCallback(
    (key: PermissionKey | string) => {
      if (!permissions) return [];
      return permissions.groups.filter((g) => g.permissionKeys.includes(key));
    },
    [permissions],
  );

  return useMemo(
    () => ({
      permissions,
      isPending,
      refetch: () => {
        refetch();
      },
      hasPermission,
      hasSystemPermission,
      groupsWithPermission,
    }),
    [
      permissions,
      isPending,
      refetch,
      hasPermission,
      hasSystemPermission,
      groupsWithPermission,
    ],
  );
}
