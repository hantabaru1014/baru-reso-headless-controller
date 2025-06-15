import HostLogViewer from "../../components/HostLogViewer";
import { useParams } from "react-router";
import HostDetailPanel from "../../components/HostDetailPanel";

export default function HostDetail() {
  const { id } = useParams();

  return (
    <div className="container mx-auto p-4 space-y-4">
      {id ? (
        <>
          <div className="w-full">
            <HostDetailPanel hostId={id} />
          </div>
          <div className="w-full">
            <HostLogViewer hostId={id} />
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
