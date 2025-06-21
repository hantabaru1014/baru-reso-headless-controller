import React from "react";
import { Button, Skeleton } from "../ui";
import { Edit, Check, X } from "lucide-react";
import { cn } from "@/libs/cssUtils";
import { FieldHeader } from "./FieldHeader";
import { FieldFooter } from "./FieldFooter";

export interface EditableFieldBaseProps {
  label?: string;
  formId?: string;
  editing: boolean;
  onEditStart?: () => void;
  onSave?: () => void;
  onCancel?: () => void;
  readonly?: boolean;
  disabled?: boolean;
  isLoading?: boolean;
  helperText?: React.ReactNode;
  error?: string;
  children: React.ReactNode;
  className?: string;
}

export function EditableFieldBase({
  label,
  formId,
  editing,
  onEditStart,
  onSave,
  onCancel,
  readonly,
  disabled,
  isLoading,
  helperText,
  error,
  children,
  className,
}: EditableFieldBaseProps) {
  return (
    <div className={className}>
      {label && (
        <FieldHeader label={label} formId={formId} helperText={helperText} />
      )}
      <div
        className={cn(
          "flex items-start gap-2",
          !editing && "border-b-2 border-b-border",
        )}
      >
        {isLoading ? (
          <Skeleton className="h-9 w-full" />
        ) : (
          <>
            <div className="flex-1 min-h-9 flex items-center">{children}</div>
            {!readonly && (
              <div className="flex flex-col items-end">
                {editing ? (
                  <div className="flex gap-1">
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={onSave}
                      title="保存"
                      data-testid="editable-field-save-button"
                    >
                      <Check className="h-4 w-4" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={onCancel}
                      title="キャンセル"
                    >
                      <X className="h-4 w-4" />
                    </Button>
                  </div>
                ) : (
                  <Button
                    size="icon"
                    variant="ghost"
                    onClick={onEditStart}
                    title="編集"
                    disabled={disabled}
                  >
                    <Edit className="h-4 w-4" />
                  </Button>
                )}
              </div>
            )}
          </>
        )}
      </div>
      <FieldFooter error={error} />
    </div>
  );
}
