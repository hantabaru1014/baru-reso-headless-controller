import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./select";
import { Label } from "./label";
import { ReactNode, useId } from "react";

export type SelectFieldOption<V> = {
  id: string;
  value?: V;
  label: ReactNode;
};

export default function SelectField<V>({
  label,
  options,
  selectedId,
  onChange,
  readOnly,
  error,
  helperText,
  minWidth,
}: {
  label?: string;
  options: SelectFieldOption<V>[];
  selectedId: string;
  onChange: (option: SelectFieldOption<V>) => void;
  readOnly?: boolean;
  error?: boolean;
  helperText?: ReactNode;
  minWidth?: string;
}) {
  const id = useId();

  return (
    <div className="space-y-2">
      {label && <Label htmlFor={id}>{label}</Label>}
      <Select
        value={selectedId}
        onValueChange={(value) => {
          const option = options.find((o) => o.id === value);
          if (option) onChange(option);
        }}
        disabled={readOnly}
      >
        <SelectTrigger
          id={id}
          style={{ minWidth }}
          className={error ? "border-destructive" : ""}
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
      {helperText && (
        <p
          className={`text-sm ${error ? "text-destructive" : "text-muted-foreground"}`}
        >
          {helperText}
        </p>
      )}
    </div>
  );
}
