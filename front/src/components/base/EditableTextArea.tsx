import { cn } from "@/libs/cssUtils";
import { hasRichTextTags } from "@/libs/richTextUtils";
import { Textarea } from "../ui";
import { EditableFieldBase, EditableFieldBaseProps } from "./EditableFieldBase";
import { ComponentProps, useId, useState } from "react";
import { RichText } from "./RichText";

export function EditableTextArea(
  props: Omit<ComponentProps<typeof Textarea>, "onChange" | "readOnly"> &
    Pick<
      EditableFieldBaseProps,
      "label" | "readonly" | "isLoading" | "helperText"
    > & {
      onSave: (value: string) => Promise<{ ok: boolean; error?: string }>;
      richTextMode?: "raw" | "ignoreLayoutTags" | "full";
    },
) {
  const [isEditing, setIsEditing] = useState(false);
  const [editingValue, setEditingValue] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);
  const id = useId();

  const handleSave = async () => {
    setError(undefined);
    setIsSaving(true);
    try {
      const { ok, error: returnedErr } = await props.onSave(editingValue);
      if (ok) {
        setIsEditing(false);
      } else {
        setError(returnedErr);
      }
    } finally {
      setIsSaving(false);
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
      isSaving={isSaving}
      helperText={props.helperText}
      disabled={props.disabled}
      error={error}
    >
      {isEditing ? (
        <div className="w-full space-y-2">
          <Textarea
            {...props}
            id={id}
            value={isEditing ? editingValue : props.value}
            onChange={(e) => setEditingValue(e.target.value)}
            readOnly={props.readonly || !isEditing}
            className={cn(error && "border-destructive")}
          />
          {props.richTextMode !== "raw" && hasRichTextTags(editingValue) && (
            <div className="rounded border bg-muted/50 px-2 py-1 text-sm">
              <RichText
                text={editingValue}
                ignoreLayoutTags={props.richTextMode === "ignoreLayoutTags"}
              />
            </div>
          )}
        </div>
      ) : (
        <div className="flex field-sizing-content min-h-16 whitespace-pre-wrap">
          {props.richTextMode === "raw" ? (
            <span>{props.value}</span>
          ) : (
            <RichText
              text={props.value as string}
              ignoreLayoutTags={props.richTextMode === "ignoreLayoutTags"}
            />
          )}
        </div>
      )}
    </EditableFieldBase>
  );
}
