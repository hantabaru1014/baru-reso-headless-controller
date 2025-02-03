import { Grid2 } from "@mui/material";
import HeadlessAccountList from "../../components/HeadlessAccountList";

export default function HeadlessAccounts() {
  return (
    <Grid2 container spacing={2}>
      <Grid2 size={12}>
        <HeadlessAccountList />
      </Grid2>
    </Grid2>
  );
}
