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
    <div className="container mx-auto p-4 space-y-4">
      {id ? (
        <>
          <div className="w-full">
            <SessionForm sessionId={id} />
          </div>
          {data?.session?.status === SessionStatus.RUNNING && (
            <div className="w-full">
              <SessionUserList sessionId={id} />
            </div>
          )}
        </>
      ) : (
        <div className="w-full">
          <p className="text-destructive">
            NotFound: セッションが見つかりませんでした
          </p>
        </div>
      )}
    </div>
  );
}
