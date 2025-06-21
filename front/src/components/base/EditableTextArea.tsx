import { cn } from "@/libs/cssUtils";
import { Textarea } from "../ui";
import { EditableFieldBase, EditableFieldBaseProps } from "./EditableFieldBase";
import { ComponentProps, useId, useState } from "react";

export function EditableTextArea(
  props: Omit<ComponentProps<typeof Textarea>, "onChange" | "readOnly"> &
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
        <Textarea
          {...props}
          id={id}
          value={isEditing ? editingValue : props.value}
          onChange={(e) => setEditingValue(e.target.value)}
          readOnly={props.readonly || !isEditing}
          className={cn(error && "border-destructive")}
        />
      ) : (
        <div className="flex field-sizing-content min-h-16 whitespace-pre">
          {props.value}
        </div>
      )}
    </EditableFieldBase>
  );
}
