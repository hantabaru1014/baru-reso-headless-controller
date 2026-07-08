import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import SessionForm from "../../components/SessionForm";
import SessionUserList from "../../components/SessionUserList";
import ScheduledOperationList from "../../components/ScheduledOperationList";
import { useQuery } from "@connectrpc/connect-query";
import { getSessionDetails } from "../../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { SessionStatus } from "../../../pbgen/hdlctrl/v1/controller_pb";

export default function SessionDetail() {
  const { t } = useTranslation();
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
          <div className="w-full border-t pt-4">
            <ScheduledOperationList sessionId={id} />
          </div>
        </>
      ) : (
        <div className="w-full">
          <p className="text-destructive">{t("sessionDetailPage.notFound")}</p>
        </div>
      )}
    </div>
  );
}
