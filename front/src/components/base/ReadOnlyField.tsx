import { EditableFieldBase } from "./EditableFieldBase";
import React from "react";

export function ReadOnlyField({
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
    <EditableFieldBase
      label={label}
      editing={false}
      isLoading={isLoading}
      readonly
      helperText={helperText}
    >
      {value}
    </EditableFieldBase>
  );
}
