import { useQuery } from "@connectrpc/connect-query";
import { getSessionDetails } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import {
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
} from "@mui/material";
import Loading from "./Loading";
import { useEffect, useState } from "react";

export default function SessionForm({
  sessionId,
  mode,
}: {
  sessionId: string;
  mode: "detail" | "edit";
}) {
  const [selectedHost] = useAtom(selectedHostAtom);
  const { data, status } = useQuery(getSessionDetails, {
    hostId: selectedHost?.id,
    sessionId,
  });

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [maxUsers, setMaxUsers] = useState(15);
  const [accessLevel, setAccessLevel] = useState(1);

  const isEditing = mode === "edit";

  useEffect(() => {
    if (status === "success" && data && data.session) {
      setName(data.session.name);
      setDescription(data.session.description);
      setMaxUsers(data.session.startupParameters?.maxUsers || 15);
      setAccessLevel(data.session.accessLevel);
    }
  }, [status]);

  return (
    <Loading loading={status === "pending"}>
      <Stack component="form" noValidate autoComplete="off" spacing={2}>
        <TextField
          label="Name"
          fullWidth
          slotProps={{
            input: {
              readOnly: !isEditing,
            },
          }}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <TextField
          label="Description"
          multiline
          fullWidth
          slotProps={{
            input: {
              readOnly: !isEditing,
            },
          }}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <Stack direction="row" spacing={2}>
          <TextField
            label="Max Users"
            type="number"
            slotProps={{
              input: {
                readOnly: !isEditing,
              },
            }}
            value={maxUsers}
            onChange={(e) => setMaxUsers(parseInt(e.target.value))}
          />
          <FormControl>
            <InputLabel id="session-form-access-level">Access Level</InputLabel>
            <Select
              labelId="session-form-access-level"
              inputProps={{ readOnly: true }}
              value={accessLevel}
              onChange={(e) => setAccessLevel(parseInt(`${e.target.value}`))}
            >
              <MenuItem value={1}>Private</MenuItem>
              <MenuItem value={2}>LAN</MenuItem>
              <MenuItem value={3}>Contacts</MenuItem>
              <MenuItem value={4}>Contacts Plus</MenuItem>
              <MenuItem value={5}>Registered User</MenuItem>
              <MenuItem value={6}>Anyone</MenuItem>
            </Select>
          </FormControl>
        </Stack>
      </Stack>
    </Loading>
  );
}
