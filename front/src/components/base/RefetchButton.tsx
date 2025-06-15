import { RefreshCw } from "lucide-react";
import { Button } from "./button";
import { useState } from "react";

export default function RefetchButton({
  refetch,
  disabled,
}: {
  refetch: () => Promise<unknown>;
  disabled?: boolean;
}) {
  const [isLoading, setIsLoading] = useState(false);

  return (
    <Button
      size="icon"
      variant="ghost"
      aria-label="再読み込み"
      onClick={async () => {
        setIsLoading(true);
        await refetch();
        setIsLoading(false);
      }}
      disabled={disabled || isLoading}
    >
      <RefreshCw className={`h-4 w-4 ${isLoading ? "animate-spin" : ""}`} />
    </Button>
  );
}
