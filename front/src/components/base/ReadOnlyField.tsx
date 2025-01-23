import { TextField } from "@mui/material";
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
      <TextField
        label={label}
        fullWidth
        variant="standard"
        value={value}
        slotProps={{
          input: {
            readOnly: true,
          },
        }}
        helperText={helperText}
      />
    </EditableFieldBase>
  );
}
