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
import { Button, Skeleton } from "./ui";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS, groupTypeToLabel } from "../libs/permissionUtils";

export default function GroupDetailPanel({ groupId }: { groupId: string }) {
  const navigate = useNavigate();
  const { data, isPending, refetch } = useQuery(getGroup, { groupId });
  const { mutateAsync: mutateUpdate } = useMutation(updateGroup);
  const { mutateAsync: mutateDelete, isPending: isDeleting } =
    useMutation(deleteGroup);
  const { hasPermission } = usePermissions();

  if (isPending) return <Skeleton className="h-32 w-full" />;
  const group = data?.group;
  if (!group)
    return <p className="text-destructive">グループが見つかりませんでした</p>;

  const canEdit =
    group.type !== GroupType.PERSONAL &&
    group.type !== GroupType.SYSTEM &&
    hasPermission(group.id, PERMISSION_KEYS.GROUP_EDIT);
  const canDelete =
    group.type === GroupType.NORMAL &&
    hasPermission(group.id, PERMISSION_KEYS.GROUP_EDIT);

  return (
    <div className="space-y-4">
      <EditableTextField
        label="グループ名"
        value={group.name}
        readonly={!canEdit}
        onSave={async (v) => {
          try {
            await mutateUpdate({ groupId: group.id, name: v });
            toast.success("グループ名を更新しました");
            refetch();
            return { ok: true };
          } catch (e) {
            return {
              ok: false,
              error: e instanceof Error ? e.message : "更新に失敗しました",
            };
          }
        }}
      />
      <ReadOnlyField label="種別" value={groupTypeToLabel(group.type)} />
      <ReadOnlyField label="ID" value={group.id} />
      <ReadOnlyField
        label="作成日時"
        value={group.createdAt?.seconds.toString() ?? ""}
      />
      {canDelete && (
        <div className="flex justify-end">
          <PermissionGuardedButton
            allowed
            variant="destructive"
            disabled={isDeleting}
            onClick={async () => {
              if (
                !confirm(
                  `グループ "${group.name}" を削除します。よろしいですか?`,
                )
              )
                return;
              try {
                await mutateDelete({ groupId: group.id });
                toast.success("グループを削除しました");
                navigate("/groups");
              } catch (e) {
                toast.error(
                  e instanceof Error
                    ? e.message
                    : "グループの削除に失敗しました",
                );
              }
            }}
          >
            グループを削除
          </PermissionGuardedButton>
        </div>
      )}
      {!canDelete &&
        group.type === GroupType.NORMAL &&
        hasPermission(group.id, PERMISSION_KEYS.GROUP_EDIT) === false && (
          <p className="text-muted-foreground text-sm">
            このグループを削除する権限がありません
          </p>
        )}
      {group.type === GroupType.PERSONAL && (
        <p className="text-muted-foreground text-sm">
          personal グループは削除・編集できません
        </p>
      )}
      <Button variant="outline" onClick={() => navigate("/groups")}>
        一覧に戻る
      </Button>
    </div>
  );
}
