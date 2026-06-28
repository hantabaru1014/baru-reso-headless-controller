import { Navigate } from "react-router";
import GroupList from "../../components/GroupList";
import { usePermissions } from "../../hooks/usePermissions";
import { PERMISSION_KEYS } from "../../libs/permissionUtils";

/**
 * 全グループの閲覧 (system:group.list 保持者向け).
 * GroupList の listGroups RPC は system:group.list を持つ場合に全件を返す仕様なので、
 * 通常の GroupList をそのまま再利用する.
 */
export default function AdminGroupsPage() {
  const { hasSystemPermission, isPending } = usePermissions();

  if (isPending) return null;
  if (!hasSystemPermission(PERMISSION_KEYS.SYSTEM_GROUP_LIST)) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="container mx-auto p-4 space-y-4">
      <p className="text-muted-foreground text-sm">
        システム上の全グループを表示しています。
      </p>
      <GroupList />
    </div>
  );
}
