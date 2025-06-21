import { ComponentProps, ReactNode, useId } from "react";
import { Textarea } from "../ui";
import { FieldHeader } from "./FieldHeader";
import { FieldFooter } from "./FieldFooter";

export function TextareaField({
  label,
  error,
  helperText,
  id,
  ...props
}: ComponentProps<typeof Textarea> & {
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
      <Textarea
        {...props}
        id={formId}
        className={error ? "border-destructive" : ""}
      />
      <FieldFooter error={error} />
    </div>
  );
}
