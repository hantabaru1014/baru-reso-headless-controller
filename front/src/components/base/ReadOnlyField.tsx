import { Input, Label } from "../ui";
import EditableFieldBase from "./EditableFieldBase";
import React from "react";

export default function ReadOnlyField({
  label,
  value,
  isLoading,
  helperText,
}: {
  label: string;
  value?: string | number;
  isLoading?: boolean;
  helperText?: React.ReactNode;
}) {
  return (
    <EditableFieldBase editing={false} isLoading={isLoading} readonly>
      <div className="space-y-2">
        <Label>{label}</Label>
        <Input value={value || ""} readOnly className="bg-muted" />
        {helperText && (
          <p className="text-sm text-muted-foreground">{helperText}</p>
        )}
      </div>
    </EditableFieldBase>
  );
}
