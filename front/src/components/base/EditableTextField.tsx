import { Input, Label } from "../ui";
import EditableFieldBase from "./EditableFieldBase";
import { ComponentProps, useState } from "react";

export default function EditableTextField(
  props: Omit<ComponentProps<typeof Input>, "onSave" | "onChange"> & {
    label?: string;
    readonly?: boolean;
    isLoading?: boolean;
    helperText?: string;
    onSave: (value: string) => Promise<{ ok: boolean; error?: string }>;
  },
) {
  const [isEditing, setIsEditing] = useState(false);
  const [editingValue, setEditingValue] = useState("");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const handleSave = async () => {
    setErrorMessage(null);
    const { ok, error } = await props.onSave(editingValue);
    if (ok) {
      setIsEditing(false);
    } else {
      setErrorMessage(error ?? null);
    }
  };

  const handleEditStart = () => {
    setEditingValue((props.value as string) || "");
    setIsEditing(true);
  };

  const handleCancel = () => {
    setIsEditing(false);
    setEditingValue((props.value as string) || "");
    setErrorMessage(null);
  };

  return (
    <EditableFieldBase
      editing={isEditing}
      onEditStart={handleEditStart}
      onSave={handleSave}
      onCancel={handleCancel}
      readonly={props.readonly}
      isLoading={props.isLoading}
    >
      <div className="space-y-2">
        {props.label && <Label>{props.label}</Label>}
        <Input
          {...props}
          value={isEditing ? editingValue : props.value}
          onChange={(e) => setEditingValue(e.target.value)}
          readOnly={props.readonly || !isEditing}
          className={`${errorMessage ? "border-destructive" : ""} ${!isEditing ? "bg-muted" : ""}`}
        />
        {(errorMessage || props.helperText) && (
          <p
            className={`text-sm ${errorMessage ? "text-destructive" : "text-muted-foreground"}`}
          >
            {errorMessage ?? props.helperText}
          </p>
        )}
      </div>
    </EditableFieldBase>
  );
}
