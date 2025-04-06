import { Grid2, Typography } from "@mui/material";
import { useParams } from "react-router";
import SessionForm from "../../components/SessionForm";
import SessionUserList from "../../components/SessionUserList";
import { useQuery } from "@connectrpc/connect-query";
import { getSessionDetails } from "../../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { SessionStatus } from "../../../pbgen/hdlctrl/v1/controller_pb";

export default function SessionDetail() {
  const { id } = useParams();
  const { data } = useQuery(getSessionDetails, {
    sessionId: id,
  });

  return (
    <Grid2 container spacing={2}>
      {id ? (
        <>
          <Grid2 size={12}>
            <SessionForm sessionId={id} />
          </Grid2>
          {data?.session?.status === SessionStatus.RUNNING && (
            <Grid2 size={12}>
              <SessionUserList sessionId={id} />
            </Grid2>
          )}
        </>
      ) : (
        <Grid2 size={12}>
          <Typography>NotFound: セッションが見つかりませんでした</Typography>
        </Grid2>
      )}
    </Grid2>
  );
}
