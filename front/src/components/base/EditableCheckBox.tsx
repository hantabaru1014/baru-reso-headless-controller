import React, { useState } from "react";
import EditableFieldBase from "./EditableFieldBase";
import { Checkbox } from "./checkbox";
import { Input } from "./input";
import { Label } from "./label";

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
        <div className="space-y-2">
          <div className="flex items-center space-x-2">
            <Checkbox
              id="editable-checkbox"
              checked={editingValue}
              onCheckedChange={(checked) => setEditingValue(checked === true)}
              disabled={disabled}
            />
            <Label htmlFor="editable-checkbox">{label}</Label>
          </div>
          {(errorMessage || helperText) && (
            <p
              className={`text-sm ${errorMessage ? "text-destructive" : "text-muted-foreground"}`}
            >
              {errorMessage ?? helperText}
            </p>
          )}
        </div>
      ) : (
        <div className="space-y-2">
          {label && <Label>{label}</Label>}
          <Input
            value={String(checked ? trueValueText : falseValueText)}
            readOnly
            className="bg-muted"
          />
          {helperText && (
            <p className="text-sm text-muted-foreground">{helperText}</p>
          )}
        </div>
      )}
    </EditableFieldBase>
  );
}
