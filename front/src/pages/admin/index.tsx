import { Link } from "react-router";
import { usePermissions } from "../../hooks/usePermissions";
import { PERMISSION_KEYS } from "../../libs/permissionUtils";
import { Button } from "../../components/ui";

/**
 * システム管理メニュー画面.
 * 持っているシステム権限に応じてリンク先を表示する.
 */
export default function AdminIndex() {
  const { hasSystemPermission } = usePermissions();

  const sections: { label: string; to: string; permission: string }[] = [
    {
      label: "ユーザー管理",
      to: "/admin/users",
      permission: PERMISSION_KEYS.SYSTEM_USER_LIST,
    },
    {
      label: "全グループ閲覧",
      to: "/admin/groups",
      permission: PERMISSION_KEYS.SYSTEM_GROUP_LIST,
    },
    {
      label: "グローバルロール管理",
      to: "/admin/roles",
      permission: PERMISSION_KEYS.SYSTEM_ROLE_MANAGE,
    },
  ];

  const available = sections.filter((s) => hasSystemPermission(s.permission));

  return (
    <div className="container mx-auto p-4 space-y-4">
      <p className="text-muted-foreground text-sm">
        システム全体の管理機能です。表示される項目は保持しているシステム権限により変わります。
      </p>
      {available.length === 0 ? (
        <p className="text-destructive text-sm">
          利用可能なシステム管理機能がありません
        </p>
      ) : (
        <div className="flex flex-col gap-2 max-w-sm">
          {available.map((s) => (
            <Button key={s.to} asChild variant="outline">
              <Link to={s.to}>{s.label}</Link>
            </Button>
          ))}
        </div>
      )}
    </div>
  );
}
