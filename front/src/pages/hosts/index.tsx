import { Grid2 } from "@mui/material";
import HostList from "../../components/HostList";

export default function Hosts() {
  return (
    <Grid2 container spacing={2}>
      <Grid2 size={12}>
        <HostList />
      </Grid2>
    </Grid2>
  );
}
