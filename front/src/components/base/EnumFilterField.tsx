import { useTranslation } from "react-i18next";
import { SelectField } from "./SelectField";

/** SelectField は id (文字列) で選択状態を持つので、enum 値から導出する. */
const filterId = (value?: number) =>
  value === undefined ? "ALL" : String(value);

/**
 * enum 値を選ぶ一覧フィルタ. 「全て」を undefined として扱う.
 */
export function EnumFilterField<V extends number>({
  label,
  values,
  selected,
  toLabel,
  onChange,
}: {
  label: string;
  values: V[];
  selected?: V;
  toLabel: (v: V) => string;
  onChange: (v?: V) => void;
}) {
  const { t } = useTranslation();
  const options = [
    { id: filterId(), value: undefined, label: t("common.all") },
    ...values.map((v) => ({ id: filterId(v), value: v, label: toLabel(v) })),
  ];

  return (
    <SelectField
      label={label}
      options={options}
      selectedId={filterId(selected)}
      onChange={(o) => onChange(o.value)}
    />
  );
}
