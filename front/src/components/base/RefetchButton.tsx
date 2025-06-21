import { RefreshCw } from "lucide-react";
import { Button } from "../ui";
import { ComponentProps, useState } from "react";
import { cn } from "@/libs/cssUtils";

export function RefetchButton({
  refetch,
  disabled,
  size = "icon",
}: {
  refetch: () => Promise<unknown>;
  disabled?: boolean;
} & Pick<ComponentProps<typeof Button>, "size">) {
  const [isLoading, setIsLoading] = useState(false);

  return (
    <Button
      size={size}
      variant="ghost"
      title="再読み込み"
      onClick={async () => {
        setIsLoading(true);
        await refetch();
        setIsLoading(false);
      }}
      disabled={disabled || isLoading}
    >
      <RefreshCw className={cn("h-4 w-4", isLoading && "animate-spin")} />
    </Button>
  );
}
