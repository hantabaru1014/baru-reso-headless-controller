import { useMutation } from "@connectrpc/connect-query";
import { Button, Card, CardContent, Typography } from "@mui/material";
import { getHeadlessHostLogs } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useEffect, useRef, useState } from "react";
import { GetHeadlessHostLogsResponse_Log } from "../../pbgen/hdlctrl/v1/controller_pb";

export default function HostLogViewer({
  hostId,
  height = "30rem",
}: {
  hostId: string;
  height?: string;
}) {
  const { mutateAsync } = useMutation(getHeadlessHostLogs);
  const [logs, setLogs] = useState<GetHeadlessHostLogsResponse_Log[]>([]);
  const scrollAreaRef = useRef<HTMLDivElement>(null);

  const fetchLogs = async (mode: "init" | "until" | "since") => {
    try {
      const query =
        mode === "init"
          ? { case: "limit" as const, value: 100 }
          : mode === "until"
            ? { case: "until" as const, value: logs[0].timestamp }
            : {
                case: "since" as const,
                value: logs[logs.length - 1].timestamp,
              };
      if (query.value == null) {
        return;
      }

      const data = await mutateAsync({
        hostId,
        // @ts-expect-error valueがundefinedではないということをわかってくれない
        query,
      });
      const newLogs =
        mode === "init"
          ? data.logs
          : mode === "since"
            ? logs.concat(data.logs)
            : data.logs.concat(logs);
      setLogs(newLogs);
      setTimeout(() => {
        if (scrollAreaRef.current) {
          switch (mode) {
            case "init":
              scrollAreaRef.current.scrollTop =
                scrollAreaRef.current.scrollHeight;
              break;
            case "until":
              scrollAreaRef.current.scrollTop = 0;
              break;
            case "since":
              scrollAreaRef.current.scrollTop =
                scrollAreaRef.current.scrollHeight;
              break;
          }
        }
      }, 100);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    fetchLogs("init");
  }, []);

  return (
    <Card sx={{ height }}>
      <CardContent sx={{ position: "relative", height }}>
        <div
          ref={scrollAreaRef}
          style={{
            position: "absolute",
            top: 0,
            right: 0,
            bottom: 0,
            left: 0,
            overflowY: "scroll",
          }}
        >
          <Button onClick={() => fetchLogs("until")} variant="text">
            以前のログを取得
          </Button>
          {logs.map((log, i) => (
            <div key={i}>
              <Typography color={log.isError ? "error" : "textPrimary"}>
                {log.body}
              </Typography>
            </div>
          ))}
          <Button onClick={() => fetchLogs("since")} variant="text">
            新しいログを取得
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
