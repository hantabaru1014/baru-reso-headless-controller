import { ComponentProps, useId } from "react";
import {
  Button,
  Checkbox,
  Label,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "../ui";
import { FieldFooter } from "./FieldFooter";
import { CircleHelp } from "lucide-react";

export function CheckboxField({
  label,
  error,
  helperText,
  id,
  ...props
}: ComponentProps<typeof Checkbox> & {
  label?: string;
  error?: string;
  helperText?: React.ReactNode;
}) {
  const formId = id ?? useId();

  return (
    <div className="pt-6">
      <div className="flex items-center space-x-2">
        <Checkbox {...props} id={formId} />
        <Label htmlFor={formId}>{label}</Label>
        {helperText && (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="smIcon">
                <CircleHelp />
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>{helperText}</p>
            </TooltipContent>
          </Tooltip>
        )}
      </div>
      <FieldFooter error={error} />
    </div>
  );
}
