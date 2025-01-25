import { TextField } from "@mui/material";
import EditableFieldBase from "./EditableFieldBase";
import { ComponentProps, useState } from "react";

export default function EditableTextField(
  props: ComponentProps<typeof TextField> & {
    readonly?: boolean;
    isLoading?: boolean;
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
      <TextField
        {...props}
        fullWidth
        variant={isEditing ? "filled" : "standard"}
        value={isEditing ? editingValue : props.value}
        onChange={(e) => setEditingValue(e.target.value)}
        slotProps={{
          input: {
            readOnly: props.readonly || !isEditing,
          },
        }}
        error={!!errorMessage}
        helperText={errorMessage ?? props.helperText}
      />
    </EditableFieldBase>
  );
}
