import * as React from "react";
import { ChevronDownIcon } from "lucide-react";
import { Button, type buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/libs/cssUtils";
import type { VariantProps } from "class-variance-authority";

export interface SplitButtonProps
  extends React.ComponentProps<"button">,
    VariantProps<typeof buttonVariants> {
  children?: React.ReactNode;
  dropdownContent?: React.ReactNode;
  onMainClick?: () => void;
  disabled?: boolean;
  mainDisabled?: boolean;
  dropdownDisabled?: boolean;
  className?: string;
}

function SplitButton({
  children,
  dropdownContent,
  onMainClick,
  disabled = false,
  mainDisabled,
  dropdownDisabled,
  variant = "default",
  size = "default",
  className,
  ...props
}: SplitButtonProps) {
  const isMainDisabled = disabled || mainDisabled;
  const isDropdownDisabled = disabled || dropdownDisabled;

  return (
    <div className={cn("flex", className)}>
      <Button
        variant={variant}
        size={size}
        disabled={isMainDisabled}
        onClick={onMainClick}
        className="rounded-r-none border-r-0"
        {...props}
      >
        {children}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant={variant}
            size={size}
            disabled={isDropdownDisabled}
            className="rounded-l-none px-2"
          >
            <ChevronDownIcon className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {dropdownContent}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

export { SplitButton };