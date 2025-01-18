import {
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  FormHelperText,
} from "@mui/material";
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
}: {
  label?: string;
  options: SelectFieldOption<V>[];
  selectedId: string;
  onChange: (option: SelectFieldOption<V>) => void;
  readOnly?: boolean;
  error?: boolean;
  helperText?: ReactNode;
}) {
  const id = useId();

  return (
    <FormControl variant="filled" error={error}>
      <InputLabel id={id}>{label}</InputLabel>
      <Select labelId={id} value={selectedId} readOnly={readOnly} autoWidth>
        {options.map((option) => (
          <MenuItem
            key={option.id}
            value={option.id}
            onClick={() => onChange(option)}
          >
            {option.label}
          </MenuItem>
        ))}
      </Select>
      <FormHelperText error={error}>{helperText}</FormHelperText>
    </FormControl>
  );
}
