import { ComponentProps, useId } from "react";
import { FieldHeader } from "./FieldHeader";
import { FieldFooter } from "./FieldFooter";
import { RadioGroup, RadioGroupItem } from "../ui";

export function RadioGroupField({
  label,
  error,
  helperText,
  options,
  ...props
}: ComponentProps<typeof RadioGroup> & {
  label?: string;
  error?: string;
  helperText?: React.ReactNode;
  options: { label: string; value: string }[];
}) {
  return (
    <div>
      {label && <FieldHeader label={label} helperText={helperText} />}
      <RadioGroup {...props}>
        {options.map((option) => {
          const id = useId();
          return (
            <div className="flex items-center space-x-2" key={option.value}>
              <RadioGroupItem value={option.value} id={id} />
              <label htmlFor={id}>{option.label}</label>
            </div>
          );
        })}
      </RadioGroup>
      <FieldFooter error={error} />
    </div>
  );
}
