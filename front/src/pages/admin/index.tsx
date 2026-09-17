import { Navigate } from "react-router";
import { useTranslation } from "react-i18next";
import { usePermissions } from "../../hooks/usePermissions";
import { ADMIN_SECTIONS } from "./sections";

/**
 * /admin への直接アクセス用.
 * 各画面へはサイドバーから直接遷移するので、開ける最初の画面へリダイレクトする.
 */
export default function AdminIndex() {
  const { t } = useTranslation();
  const { hasSystemPermission, isPending } = usePermissions();

  if (isPending) return null;

  const first = ADMIN_SECTIONS.find((s) =>
    s.permissions.some((p) => hasSystemPermission(p)),
  );
  if (first) return <Navigate to={first.href} replace />;

  return (
    <div className="container mx-auto p-4">
      <p className="text-destructive text-sm">
        {t("adminIndexPage.noAvailableFeatures")}
      </p>
    </div>
  );
}
