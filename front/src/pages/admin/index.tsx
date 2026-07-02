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

  // permissions のいずれかを持てばリンクを表示する.
  const sections: { label: string; to: string; permissions: string[] }[] = [
    {
      label: "ユーザー管理",
      to: "/admin/users",
      // ユーザー管理画面は list/create/delete のいずれかで開ける.
      permissions: [
        PERMISSION_KEYS.SYSTEM_USER_LIST,
        PERMISSION_KEYS.SYSTEM_USER_CREATE,
        PERMISSION_KEYS.SYSTEM_USER_DELETE,
      ],
    },
    {
      label: "全グループ閲覧",
      to: "/admin/groups",
      permissions: [PERMISSION_KEYS.SYSTEM_GROUP_LIST],
    },
    {
      label: "グローバルロール管理",
      to: "/admin/roles",
      permissions: [PERMISSION_KEYS.SYSTEM_ROLE_MANAGE],
    },
  ];

  const available = sections.filter((s) =>
    s.permissions.some((p) => hasSystemPermission(p)),
  );

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
