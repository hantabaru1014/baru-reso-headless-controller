import { useQuery } from "@connectrpc/connect-query";
import { useMemo } from "react";
import { listGroups } from "../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { GroupType } from "../../pbgen/hdlctrl/v1/permission_pb";
import { SelectField } from "./base";
import { usePermissions } from "../hooks/usePermissions";
import { PermissionKey } from "../libs/permissionUtils";
import { groupTypeToLabel } from "../libs/permissionUtils";
import { useTranslation } from "react-i18next";

/**
 * リソース作成フォーム用のグループ選択フィールド.
 *
 * - `requiredPermission` で指定した key を呼び出しユーザーが持つグループのみ選択肢に出る
 * - personal グループはデフォルト選択候補 (リスト先頭) として並ぶ
 * - `restrictToGroupId` を指定すると、そのグループのみに選択肢を絞る (同一グループ制約用)
 */
export function GroupSelectField({
  label,
  helperText,
  value,
  onChange,
  requiredPermission,
  restrictToGroupId,
  error,
  readOnly,
}: {
  label?: string;
  helperText?: string;
  value: string;
  onChange: (groupId: string) => void;
  requiredPermission: PermissionKey;
  restrictToGroupId?: string;
  error?: string;
  readOnly?: boolean;
}) {
  const { t } = useTranslation();
  const { data, isPending } = useQuery(listGroups, {});
  const { groupsWithPermission } = usePermissions();

  const options = useMemo(() => {
    const allowedGroupIds = new Set(
      groupsWithPermission(requiredPermission).map((g) => g.groupId),
    );
    const allGroups = data?.groups ?? [];
    let candidates = allGroups.filter(
      (g) => g.type !== GroupType.SYSTEM && allowedGroupIds.has(g.id),
    );
    if (restrictToGroupId) {
      candidates = candidates.filter((g) => g.id === restrictToGroupId);
    }
    // personal を先頭に
    candidates.sort((a, b) => {
      if (a.type === b.type) return a.name.localeCompare(b.name);
      return a.type === GroupType.PERSONAL ? -1 : 1;
    });
    return candidates.map((g) => ({
      id: g.id,
      label: `${g.name} (${groupTypeToLabel(g.type)})`,
    }));
  }, [
    data?.groups,
    groupsWithPermission,
    requiredPermission,
    restrictToGroupId,
  ]);

  return (
    <SelectField
      label={label ?? t("groupSelectField.belongingGroup")}
      helperText={helperText}
      options={options}
      selectedId={value}
      onChange={(option) => onChange(option.id)}
      error={
        error ??
        (!isPending && options.length === 0
          ? t("groupSelectField.noSelectableGroups")
          : undefined)
      }
      readOnly={readOnly || isPending}
    />
  );
}
