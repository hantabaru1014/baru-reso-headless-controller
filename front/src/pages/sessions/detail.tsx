import { Button, Grid2, Typography } from "@mui/material";
import { useNavigate, useParams } from "react-router";
import SessionForm from "../../components/SessionForm";
import SessionControlButtons from "../../components/SessionControlButtons";
import SessionUserList from "../../components/SessionUserList";

export default function SessionDetail() {
  const { id } = useParams();
  const navigate = useNavigate();

  return (
    <Grid2 container spacing={2}>
      <Grid2 size={12}>
        <Button variant="text" onClick={() => navigate("/sessions")}>
          Back to Session List
        </Button>
      </Grid2>
      {id ? (
        <>
          <Grid2 size={12} container sx={{ justifyContent: "flex-end" }}>
            <SessionControlButtons sessionId={id} />
          </Grid2>
          <Grid2 size={12}>
            <SessionForm sessionId={id} />
          </Grid2>
          <Grid2 size={12}>
            <SessionUserList sessionId={id} />
          </Grid2>
        </>
      ) : (
        <Grid2 size={12}>
          <Typography>NotFound: セッションが見つかりませんでした</Typography>
        </Grid2>
      )}
    </Grid2>
  );
}
