import { Refresh } from "@mui/icons-material";
import { IconButton } from "@mui/material";
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
    <IconButton
      aria-label="再読み込み"
      onClick={async () => {
        setIsLoading(true);
        await refetch();
        setIsLoading(false);
      }}
      disabled={disabled || isLoading}
    >
      <Refresh />
    </IconButton>
  );
}
