import { Button, Grid2 } from "@mui/material";
import { useNavigate, useParams } from "react-router";
import SessionForm from "../../components/SessionForm";

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
      <Grid2 size={12}>
        {id && <SessionForm sessionId={id} mode="detail" />}
      </Grid2>
    </Grid2>
  );
}
