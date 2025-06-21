import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui";
import { ReactNode, useId } from "react";
import { FieldHeader } from "./FieldHeader";
import { FieldFooter } from "./FieldFooter";
import { cn } from "@/libs/cssUtils";

export type SelectFieldOption<V> = {
  id: string;
  value?: V;
  label: ReactNode;
};

export function SelectField<V>({
  id,
  label,
  options,
  selectedId,
  onChange,
  readOnly,
  error,
  helperText,
  minWidth,
  className,
}: {
  id?: string;
  label?: string;
  options: SelectFieldOption<V>[];
  selectedId: string;
  onChange: (option: SelectFieldOption<V>) => void;
  readOnly?: boolean;
  error?: string;
  helperText?: ReactNode;
  minWidth?: string;
  className?: string;
}) {
  const formId = id ?? useId();

  return (
    <div className={className}>
      {label && (
        <FieldHeader formId={formId} label={label} helperText={helperText} />
      )}
      <Select
        value={selectedId}
        onValueChange={(value) => {
          const option = options.find((o) => o.id === value);
          if (option) onChange(option);
        }}
        disabled={readOnly}
      >
        <SelectTrigger
          id={formId}
          style={{ minWidth }}
          className={cn("w-full", error && "border-destructive")}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.id} value={option.id}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <FieldFooter error={error} />
    </div>
  );
}
