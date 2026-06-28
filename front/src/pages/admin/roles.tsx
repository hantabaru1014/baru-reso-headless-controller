import { Navigate } from "react-router";
import RoleList from "../../components/RoleList";
import { usePermissions } from "../../hooks/usePermissions";
import { PERMISSION_KEYS } from "../../libs/permissionUtils";
import { RoleScope } from "../../../pbgen/hdlctrl/v1/permission_pb";

export default function AdminRolesPage() {
  const { hasSystemPermission, isPending } = usePermissions();

  if (isPending) return null;
  if (!hasSystemPermission(PERMISSION_KEYS.SYSTEM_ROLE_MANAGE)) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="container mx-auto p-4 space-y-4">
      <p className="text-muted-foreground text-sm">
        グローバルカスタムロール (全グループで割り当て可能なロール)
        を管理します。
      </p>
      <RoleList canManage scope={RoleScope.NORMAL} />
    </div>
  );
}
