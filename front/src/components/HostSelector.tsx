import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import { useQuery } from "@connectrpc/connect-query";
import { listHeadlessHost } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useEffect } from "react";
import SelectField from "./base/SelectField";
import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { Skeleton } from "@mui/material";

export default function HostSelector() {
  const [selectedHost, setSelectedHost] = useAtom(selectedHostAtom);
  const { data, status } = useQuery(listHeadlessHost);

  useEffect(() => {
    if (status === "success" && !selectedHost && data?.hosts.length > 0) {
      setSelectedHost({
        id: data?.hosts[0].id,
        name: data?.hosts[0].name,
      });
    } else if (status === "error") {
      setSelectedHost(null);
    }
  }, [status, data]);

  if (status === "success") {
    return (
      <SelectField
        label="Host"
        options={
          data?.hosts
            .filter((host) => host.status === HeadlessHostStatus.RUNNING)
            .map((host) => ({
              id: host.id,
              label: `${host.name} (${host.id.slice(0, 6)})`,
              value: host,
            })) ?? []
        }
        selectedId={selectedHost?.id || ""}
        onChange={(option) => setSelectedHost(option.value ?? null)}
      />
    );
  } else {
    return <Skeleton variant="rectangular" />;
  }
}
