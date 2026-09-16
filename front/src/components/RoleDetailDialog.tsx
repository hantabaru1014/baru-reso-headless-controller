import { useQuery } from "@connectrpc/connect-query";
import { useTranslation } from "react-i18next";
import { listPermissions } from "../../pbgen/hdlctrl/v1/permission-RoleService_connectquery";
import { Role } from "../../pbgen/hdlctrl/v1/permission_pb";
import {
  Button,
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui";
import { Loading, ReadOnlyField } from "./base";
import { PermissionKeyCheckList } from "./PermissionKeyCheckList";
import {
  PERMISSIONS_STALE_TIME,
  roleScopeToLabel,
} from "../libs/permissionUtils";

/**
 * ロールの内容 (名前 / スコープ / 種別 / 付与パーミッション) を閲覧専用で表示するダイアログ.
 * 組込ロールのように編集できないロールの中身を確認する用途.
 */
export function RoleDetailDialog({
  role,
  open,
  onClose,
}: {
  role: Role;
  open: boolean;
  onClose?: () => void;
}) {
  const { t } = useTranslation();
  const { data: permsData, isPending } = useQuery(
    listPermissions,
    { scope: role.scope },
    { staleTime: PERMISSIONS_STALE_TIME },
  );

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose?.();
      }}
    >
      <DialogContent className="sm:max-w-[500px] max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("roleList.roleDetail")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <ReadOnlyField label={t("roleList.roleName")} value={role.name} />
          <ReadOnlyField
            label={t("roleList.scope")}
            value={roleScopeToLabel(role.scope)}
          />
          <ReadOnlyField
            label={t("roleList.type")}
            value={
              role.isBuiltin ? t("roleList.builtin") : t("roleList.custom")
            }
          />
          <div className="space-y-2">
            <p className="text-sm font-medium">
              {t("roleList.grantedPermissions")}
            </p>
            <Loading loading={isPending}>
              {/* 読み込み中はリストが空で高さ 0 になりスピナーがラベルに被るため最低高さを確保する */}
              <div className="min-h-12">
                <PermissionKeyCheckList
                  permissions={permsData?.permissions ?? []}
                  value={role.permissionKeys}
                  readOnly
                />
              </div>
            </Loading>
          </div>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">{t("common.close")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
