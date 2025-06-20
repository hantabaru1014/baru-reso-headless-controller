import { ComponentProps, ReactNode, useId } from "react";
import { Input } from "../ui";
import { FieldHeader } from "./FieldHeader";
import { FieldFooter } from "./FieldFooter";

export function TextField({
  label,
  error,
  helperText,
  id,
  ...props
}: ComponentProps<typeof Input> & {
  label?: string;
  error?: string;
  helperText?: ReactNode;
}) {
  const formId = id ?? useId();

  return (
    <div>
      {label && (
        <FieldHeader formId={formId} label={label} helperText={helperText} />
      )}
      <Input
        {...props}
        id={formId}
        className={error ? "border-destructive" : ""}
      />
      <FieldFooter error={error} />
    </div>
  );
}
