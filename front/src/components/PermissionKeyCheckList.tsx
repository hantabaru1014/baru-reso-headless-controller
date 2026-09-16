import { Checkbox } from "./ui";
import { PermissionKey } from "../../pbgen/hdlctrl/v1/permission_pb";
import { permissionKeyToLabel } from "../libs/permissionUtils";

/**
 * permission_key のチェックボックス一覧.
 *
 * ロール編集ダイアログ (編集可) とロール詳細ダイアログ (閲覧のみ) で共用する.
 * `readOnly` のときはチェック状態の表示のみで操作できない.
 */
export function PermissionKeyCheckList({
  permissions,
  value,
  onChange,
  readOnly = false,
}: {
  permissions: PermissionKey[];
  value: string[];
  onChange?: (next: string[]) => void;
  readOnly?: boolean;
}) {
  return (
    <div className="space-y-2">
      {permissions.map((p) => {
        const checked = value.includes(p.key);
        return (
          <label
            key={p.key}
            className={`flex items-center gap-2 ${readOnly ? "" : "cursor-pointer"}`}
          >
            <Checkbox
              checked={checked}
              disabled={readOnly}
              onCheckedChange={(c) => {
                if (readOnly) return;
                const next = c
                  ? [...value, p.key]
                  : value.filter((k) => k !== p.key);
                onChange?.(next);
              }}
            />
            <span className="text-sm">
              {permissionKeyToLabel(p.key)}{" "}
              <span className="text-muted-foreground font-mono text-xs">
                ({p.key})
              </span>
            </span>
          </label>
        );
      })}
    </div>
  );
}
