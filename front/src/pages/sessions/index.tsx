import { Grid2, Typography } from "@mui/material";
import SessionList from "../../components/SessionList";
import { useAtom } from "jotai";
import { selectedHostAtom } from "../../atoms/selectedHostAtom";

export default function Sessions() {
  const [selectedHost] = useAtom(selectedHostAtom);

  return (
    <Grid2 container spacing={2}>
      <Grid2 size={12}>
        {selectedHost ? (
          <SessionList />
        ) : (
          <Typography>ホストを選択してください</Typography>
        )}
      </Grid2>
    </Grid2>
  );
}
