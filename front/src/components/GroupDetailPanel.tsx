import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import {
  deleteGroup,
  getGroup,
  updateGroup,
} from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { GroupType } from "../../pbgen/hdlctrl/v1/permission_pb";
import { EditableTextField, ReadOnlyField } from "./base";
import { Skeleton } from "./ui";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS, groupTypeToLabel } from "../libs/permissionUtils";
import { formatTimestamp } from "../libs/datetimeUtils";
import { useTranslation } from "react-i18next";

export default function GroupDetailPanel({ groupId }: { groupId: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data, isPending, refetch } = useQuery(getGroup, { groupId });
  const { mutateAsync: mutateUpdate } = useMutation(updateGroup);
  const { mutateAsync: mutateDelete, isPending: isDeleting } =
    useMutation(deleteGroup);
  const { hasPermission } = usePermissions();

  if (isPending) return <Skeleton className="h-32 w-full" />;
  const group = data?.group;
  if (!group)
    return <p className="text-destructive">{t("groupDetailPanel.notFound")}</p>;

  const canEdit =
    group.type !== GroupType.PERSONAL &&
    group.type !== GroupType.SYSTEM &&
    hasPermission(group.id, PERMISSION_KEYS.GROUP_EDIT);
  const canDelete =
    group.type === GroupType.NORMAL &&
    hasPermission(group.id, PERMISSION_KEYS.GROUP_EDIT);

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <EditableTextField
          label={t("groupDetailPanel.groupName")}
          value={group.name}
          readonly={!canEdit}
          onSave={async (v) => {
            try {
              await mutateUpdate({ groupId: group.id, name: v });
              toast.success(t("groupDetailPanel.nameUpdated"));
              refetch();
              return { ok: true };
            } catch (e) {
              return {
                ok: false,
                error:
                  e instanceof Error
                    ? e.message
                    : t("groupDetailPanel.updateFailed"),
              };
            }
          }}
        />
        <div className="flex items-start justify-end">
          {canDelete && (
            <PermissionGuardedButton
              allowed
              variant="destructive"
              disabled={isDeleting}
              onClick={async () => {
                if (
                  !confirm(
                    t("groupDetailPanel.confirmDelete", { name: group.name }),
                  )
                )
                  return;
                try {
                  await mutateDelete({ groupId: group.id });
                  toast.success(t("groupDetailPanel.deleted"));
                  navigate("/groups");
                } catch (e) {
                  toast.error(
                    e instanceof Error
                      ? e.message
                      : t("groupDetailPanel.deleteFailed"),
                  );
                }
              }}
            >
              {t("groupDetailPanel.deleteGroup")}
            </PermissionGuardedButton>
          )}
        </div>
        <ReadOnlyField
          label={t("groupDetailPanel.type")}
          value={groupTypeToLabel(group.type)}
        />
        <ReadOnlyField label="ID" value={group.id} />
        <ReadOnlyField
          label={t("groupDetailPanel.createdAt")}
          value={formatTimestamp(group.createdAt)}
        />
      </div>
      {!canDelete &&
        group.type === GroupType.NORMAL &&
        hasPermission(group.id, PERMISSION_KEYS.GROUP_EDIT) === false && (
          <p className="text-muted-foreground text-sm">
            {t("groupDetailPanel.noDeletePermission")}
          </p>
        )}
      {group.type === GroupType.PERSONAL && (
        <p className="text-muted-foreground text-sm">
          {t("groupDetailPanel.personalReadonly")}
        </p>
      )}
    </div>
  );
}
