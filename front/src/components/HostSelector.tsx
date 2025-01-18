import { useAtom } from "jotai";
import { selectedHostAtom } from "../atoms/selectedHostAtom";
import { useQuery } from "@connectrpc/connect-query";
import { listHeadlessHost } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import Loading from "./base/Loading";
import { useEffect } from "react";
import SelectField from "./base/SelectField";

export default function HostSelector() {
  const [selectedHost, setSelectedHost] = useAtom(selectedHostAtom);
  const { data, status } = useQuery(listHeadlessHost);

  useEffect(() => {
    if (status === "success" && !selectedHost && data?.hosts.length > 0) {
      setSelectedHost({
        id: data?.hosts[0].id,
        name: data?.hosts[0].name,
      });
    }
  }, [status]);

  return (
    <Loading loading={status === "pending"}>
      <SelectField
        label="Host"
        options={
          data?.hosts.map((host) => ({
            id: host.id,
            label: host.name,
            value: host,
          })) ?? []
        }
        selectedId={selectedHost?.id || ""}
        onChange={(option) => setSelectedHost(option.value ?? null)}
      />
    </Loading>
  );
}
