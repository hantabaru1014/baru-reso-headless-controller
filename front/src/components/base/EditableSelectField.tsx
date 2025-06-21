import { EditableFieldBase, EditableFieldBaseProps } from "./EditableFieldBase";
import { ComponentProps, useId, useState } from "react";
import { SelectField, type SelectFieldOption } from "./SelectField";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui";
import { cn } from "@/libs/cssUtils";

export function EditableSelectField<V>(
  props: Omit<ComponentProps<typeof SelectField<V>>, "onChange" | "readOnly"> &
    Pick<
      EditableFieldBaseProps,
      "label" | "readonly" | "isLoading" | "helperText" | "disabled"
    > & {
      onSave: (value: V) => Promise<{ ok: boolean; error?: string }>;
    },
) {
  const [isEditing, setIsEditing] = useState(false);
  const [editingValue, setEditingValue] = useState<SelectFieldOption<V>>();
  const [error, setError] = useState<string | undefined>(undefined);
  const id = useId();

  const selectedOption = props.options.find((o) => o.id === props.selectedId);

  const handleSave = async () => {
    setError(undefined);
    const { ok, error: returnedErr } = await props.onSave(
      (editingValue?.value ?? editingValue?.id) as V,
    );
    if (ok) {
      setIsEditing(false);
    } else {
      setError(returnedErr);
    }
  };

  const handleEditStart = () => {
    setEditingValue(selectedOption);
    setIsEditing(true);
  };

  const handleCancel = () => {
    setIsEditing(false);
    setEditingValue(selectedOption);
    setError(undefined);
  };

  return (
    <EditableFieldBase
      label={props.label}
      formId={id}
      editing={isEditing}
      onEditStart={handleEditStart}
      onSave={handleSave}
      onCancel={handleCancel}
      readonly={props.readonly}
      isLoading={props.isLoading}
      helperText={props.helperText}
      disabled={props.disabled}
      error={error}
    >
      {isEditing ? (
        <Select
          value={isEditing ? editingValue?.id || "" : props.selectedId}
          onValueChange={(value) => {
            const option = props.options.find((o) => o.id === value);
            if (option) setEditingValue(option);
          }}
          disabled={props.readonly || !isEditing}
        >
          <SelectTrigger
            id={id}
            className={cn("w-full", error && "border-destructive")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {props.options.map((option) => (
              <SelectItem key={option.id} value={option.id}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : (
        <span>{selectedOption?.label?.toString() || ""}</span>
      )}
    </EditableFieldBase>
  );
}
