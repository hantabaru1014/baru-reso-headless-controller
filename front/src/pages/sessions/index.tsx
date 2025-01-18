import { Grid2 } from "@mui/material";
import SessionList from "../../components/SessionList";

export default function Sessions() {
  return (
    <Grid2 container spacing={2}>
      <Grid2 size={12}>
        <SessionList />
      </Grid2>
    </Grid2>
  );
}
