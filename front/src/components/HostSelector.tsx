import { FormControl, InputLabel, MenuItem, Select } from "@mui/material";
import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import { useQuery } from "@connectrpc/connect-query";
import { listHeadlessHost } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import Loading from "./Loading";
import { useId } from "react";

export default function HostSelector() {
  const [selectedHost, setSelectedHost] = useAtom(selectedHostAtom);
  const { data, status } = useQuery(listHeadlessHost);
  const id = useId();

  return (
    <Loading loading={status === "pending"}>
      <FormControl>
        <InputLabel id={id}>Host</InputLabel>
        <Select labelId={id} value={selectedHost?.id || ""}>
          {data?.hosts.map((host) => (
            <MenuItem
              key={host.id}
              value={host.id}
              onClick={() =>
                setSelectedHost({
                  id: host.id,
                  name: host.name,
                })
              }
            >
              {host.name}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    </Loading>
  );
}
