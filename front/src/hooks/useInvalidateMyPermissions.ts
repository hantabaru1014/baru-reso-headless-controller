import { createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { getMyPermissions } from "../../pbgen/hdlctrl/v1/permission-RoleService_connectquery";

/**
 * getMyPermissions クエリを invalidate するコールバックを返す.
 * ロール/メンバー/グループの変更は自分自身の実効権限に影響しうるため利用する.
 */
export function useInvalidateMyPermissions() {
  const queryClient = useQueryClient();
  return useCallback(
    () =>
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: getMyPermissions,
          cardinality: undefined,
        }),
      }),
    [queryClient],
  );
}
