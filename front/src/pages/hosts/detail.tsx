import HostLogViewer from "../../components/HostLogViewer";
import { useParams } from "react-router";
import HostDetailPanel from "../../components/HostDetailPanel";
import { useQuery } from "@connectrpc/connect-query";
import { getHeadlessHost } from "../../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { HeadlessHostStatus } from "../../../pbgen/hdlctrl/v1/controller_pb";

export default function HostDetail() {
  const { id } = useParams();
  const { data: hostData } = useQuery(
    getHeadlessHost,
    { hostId: id ?? "" },
    { enabled: !!id },
  );

  return (
    <div className="container mx-auto p-4 space-y-4">
      {id ? (
        <>
          <div className="w-full">
            <HostDetailPanel hostId={id} />
          </div>
          <div className="w-full">
            {hostData?.host?.instanceId !== undefined && (
              <HostLogViewer
                hostId={id}
                instanceId={hostData.host.instanceId}
                tailing={
                  hostData.host.status !== HeadlessHostStatus.EXITED &&
                  hostData.host.status !== HeadlessHostStatus.CRASHED
                }
              />
            )}
          </div>
        </>
      ) : (
        <div className="w-full">
          <p className="text-destructive">
            NotFound: ホストが見つかりませんでした
          </p>
        </div>
      )}
    </div>
  );
}
