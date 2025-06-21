import { CircleHelp } from "lucide-react";
import { Button, Label, Tooltip, TooltipContent, TooltipTrigger } from "../ui";
import { cn } from "@/libs/cssUtils";

export interface FieldHeaderProps {
  label: string;
  formId?: string;
  helperText?: React.ReactNode;
  required?: boolean;
  className?: string;
}

export function FieldHeader({
  label,
  formId,
  helperText,
  required,
  className,
}: FieldHeaderProps) {
  return (
    <div className={cn("flex mb-1 gap-2", className)}>
      <Label htmlFor={formId}>{label}</Label>
      {required && <span>必須</span>}
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
  );
}
