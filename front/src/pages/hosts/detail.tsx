import { Grid2, Typography } from "@mui/material";
import HostLogViewer from "../../components/HostLogViewer";
import { useParams } from "react-router";

export default function HostDetail() {
  const { id } = useParams();

  return (
    <Grid2 container>
      {id ? (
        <Grid2 size={12}>
          <HostLogViewer hostId={id} />
        </Grid2>
      ) : (
        <Grid2 size={12}>
          <Typography>NotFound: ホストが見つかりませんでした</Typography>
        </Grid2>
      )}
    </Grid2>
  );
}
