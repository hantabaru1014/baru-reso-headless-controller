import { Input, Label } from "../ui";
import EditableFieldBase from "./EditableFieldBase";
import { ComponentProps, useState } from "react";
import SelectField, { type SelectFieldOption } from "./SelectField";

export default function EditableSelectField<V>(
  props: Omit<ComponentProps<typeof SelectField<V>>, "onChange"> & {
    onSave: (value: V) => Promise<{ ok: boolean; error?: string }>;
    isLoading?: boolean;
  },
) {
  const [isEditing, setIsEditing] = useState(false);
  const [editingValue, setEditingValue] = useState<SelectFieldOption<V>>();
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const selectedOption = props.options.find((o) => o.id === props.selectedId);

  const handleSave = async () => {
    setErrorMessage(null);
    const { ok, error } = await props.onSave(
      (editingValue?.value ?? editingValue?.id) as V,
    );
    if (ok) {
      setIsEditing(false);
    } else {
      setErrorMessage(error ?? null);
    }
  };

  const handleEditStart = () => {
    setEditingValue(selectedOption);
    setIsEditing(true);
  };

  const handleCancel = () => {
    setIsEditing(false);
    setEditingValue(selectedOption);
    setErrorMessage(null);
  };

  return (
    <EditableFieldBase
      editing={isEditing}
      onEditStart={handleEditStart}
      onSave={handleSave}
      onCancel={handleCancel}
      readonly={props.readOnly}
      isLoading={props.isLoading}
    >
      {isEditing ? (
        <SelectField
          {...props}
          selectedId={isEditing ? editingValue?.id || "" : props.selectedId}
          onChange={(option) => setEditingValue(option)}
          readOnly={props.readOnly || !isEditing}
          error={!!errorMessage}
          helperText={errorMessage ?? props.helperText}
        />
      ) : (
        <div className="space-y-2">
          {props.label && <Label>{props.label}</Label>}
          <Input
            value={selectedOption?.label?.toString() || ""}
            readOnly
            className="bg-muted"
          />
          {props.helperText && (
            <p className="text-sm text-muted-foreground">{props.helperText}</p>
          )}
        </div>
      )}
    </EditableFieldBase>
  );
}
