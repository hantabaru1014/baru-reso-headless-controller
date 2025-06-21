import { cn } from "@/libs/cssUtils";
import { Input } from "../ui";
import { EditableFieldBase, EditableFieldBaseProps } from "./EditableFieldBase";
import { ComponentProps, useId, useState } from "react";

export function EditableTextField(
  props: Omit<ComponentProps<typeof Input>, "onChange" | "readOnly"> &
    Pick<
      EditableFieldBaseProps,
      "label" | "readonly" | "isLoading" | "helperText"
    > & {
      onSave: (value: string) => Promise<{ ok: boolean; error?: string }>;
    },
) {
  const [isEditing, setIsEditing] = useState(false);
  const [editingValue, setEditingValue] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);
  const id = useId();

  const handleSave = async () => {
    setError(undefined);
    const { ok, error: returnedErr } = await props.onSave(editingValue);
    if (ok) {
      setIsEditing(false);
    } else {
      setError(returnedErr);
    }
  };

  const handleEditStart = () => {
    setEditingValue((props.value as string) || "");
    setIsEditing(true);
  };

  const handleCancel = () => {
    setIsEditing(false);
    setEditingValue((props.value as string) || "");
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
        <Input
          {...props}
          id={id}
          value={isEditing ? editingValue : props.value}
          onChange={(e) => setEditingValue(e.target.value)}
          readOnly={props.readonly || !isEditing}
          className={cn(error && "border-destructive")}
        />
      ) : (
        <span>{props.value}</span>
      )}
    </EditableFieldBase>
  );
}
