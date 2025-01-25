import React, { useState } from "react";
import EditableFieldBase from "./EditableFieldBase";
import {
  Checkbox,
  FormControlLabel,
  FormHelperText,
  TextField,
} from "@mui/material";

export default function EditableCheckBox({
  label,
  checked,
  trueValueText = "有効",
  falseValueText = "無効",
  disabled,
  readonly,
  isLoading,
  helperText,
  onSave,
}: {
  label?: React.ReactNode;
  checked?: boolean;
  trueValueText?: React.ReactNode;
  falseValueText?: React.ReactNode;
  disabled?: boolean;
  readonly?: boolean;
  isLoading?: boolean;
  helperText?: React.ReactNode;
  onSave: (checked: boolean) => Promise<{ ok: boolean; error?: string }>;
}) {
  const [isEditing, setIsEditing] = useState(false);
  const [editingValue, setEditingValue] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const handleSave = async () => {
    setErrorMessage(null);
    const { ok, error } = await onSave(editingValue);
    if (ok) {
      setIsEditing(false);
    } else {
      setErrorMessage(error ?? null);
    }
  };

  const handleEditStart = () => {
    setEditingValue(checked ?? false);
    setIsEditing(true);
  };

  const handleCancel = () => {
    setIsEditing(false);
    setEditingValue(checked ?? false);
    setErrorMessage(null);
  };

  return (
    <EditableFieldBase
      editing={isEditing}
      onEditStart={handleEditStart}
      onSave={handleSave}
      onCancel={handleCancel}
      readonly={readonly}
      isLoading={isLoading}
    >
      {isEditing ? (
        <div>
          <FormControlLabel
            label={label}
            control={
              <Checkbox
                checked={editingValue}
                onChange={(e) => setEditingValue(e.target.checked)}
              />
            }
            disabled={disabled}
          />
          <FormHelperText error={errorMessage == null}>
            {errorMessage ?? helperText}
          </FormHelperText>
        </div>
      ) : (
        <TextField
          label={label}
          fullWidth
          variant="standard"
          value={checked ? trueValueText : falseValueText}
          slotProps={{
            input: {
              readOnly: true,
            },
          }}
          helperText={helperText}
        />
      )}
    </EditableFieldBase>
  );
}
