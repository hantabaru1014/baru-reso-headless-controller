import { ComponentProps, ReactNode, useId } from "react";
import { hasRichTextTags } from "@/libs/richTextUtils";
import { Textarea } from "../ui";
import { FieldHeader } from "./FieldHeader";
import { FieldFooter } from "./FieldFooter";
import { RichText } from "./RichText";

export function TextareaField({
  label,
  error,
  helperText,
  id,
  richTextMode = "raw",
  ...props
}: ComponentProps<typeof Textarea> & {
  label?: string;
  error?: string;
  helperText?: ReactNode;
  richTextMode?: "raw" | "ignoreLayoutTags" | "full";
}) {
  const formId = id ?? useId();
  const value = typeof props.value === "string" ? props.value : "";

  return (
    <div>
      {label && (
        <FieldHeader formId={formId} label={label} helperText={helperText} />
      )}
      <div className="space-y-2">
        <Textarea
          {...props}
          id={formId}
          className={error ? "border-destructive" : ""}
        />
        {richTextMode !== "raw" && hasRichTextTags(value) && (
          <div className="rounded border bg-muted/50 px-2 py-1 text-sm">
            <RichText
              text={value}
              ignoreLayoutTags={richTextMode === "ignoreLayoutTags"}
            />
          </div>
        )}
      </div>
      <FieldFooter error={error} />
    </div>
  );
}
