import { useMutation } from "@connectrpc/connect-query";
import { Button } from "./base/button";
import { Card, CardContent, CardHeader } from "./base/card";
import { Checkbox } from "./base/checkbox";
import { Label } from "./base/label";
import { getHeadlessHostLogs } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  GetHeadlessHostLogsRequest,
  GetHeadlessHostLogsResponse_Log,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import { toast } from "sonner";

export default function HostLogViewer({
  hostId,
  height = "30rem",
}: {
  hostId: string;
  height?: string;
}) {
  const { mutateAsync } = useMutation(getHeadlessHostLogs);
  const [logs, setLogs] = useState<GetHeadlessHostLogsResponse_Log[]>([]);
  const [isFetchedUntilLogs, setIsFetchedUntilLogs] = useState(false);
  const [isTailing, setIsTailing] = useState(false);
  const scrollAreaRef = useRef<HTMLDivElement>(null);

  const fetchLogs = useCallback(
    async (mode: "init" | "until" | "since") => {
      try {
        if (mode === "until" && isFetchedUntilLogs) {
          return;
        }
        let query: GetHeadlessHostLogsRequest["query"] = {
          case: "limit" as const,
          value: 100,
        };
        if (mode === "since" && logs.length > 0) {
          const lastTimestamp = logs[logs.length - 1].timestamp!;
          query = {
            case: "since" as const,
            value: {
              ...lastTimestamp,
              // sinceに最後のログの時間をそのまま入れてしまうと、そのログも取得されてしまう対策
              seconds: lastTimestamp.seconds + BigInt(1),
              nanos: 0,
            },
          };
        }
        if (mode === "until" && logs.length > 0) {
          query = {
            case: "until" as const,
            // こっちはどうせ1回だけなので、取得した後にけずる
            value: logs[0].timestamp!,
          };
        }

        const data = await mutateAsync({
          hostId,
          query,
        });
        if (mode === "until") {
          setIsFetchedUntilLogs(true);
        }
        if (data.logs.length === 0) {
          return;
        }

        setLogs((prev) =>
          mode === "init"
            ? data.logs
            : mode === "since"
              ? prev.concat(data.logs)
              : data.logs.slice(0, -1).concat(prev),
        );
        setTimeout(() => {
          if (scrollAreaRef.current) {
            switch (mode) {
              case "init":
              case "since":
                scrollAreaRef.current.scrollTo(
                  scrollAreaRef.current.scrollLeft,
                  scrollAreaRef.current.scrollHeight,
                );
                break;
              case "until":
                scrollAreaRef.current.scrollTo(0, 0);
                break;
            }
          }
        }, 10);
      } catch (e) {
        toast.error(`ログ取得エラー: ${e instanceof Error ? e.message : e}`);
      }
    },
    [
      isFetchedUntilLogs,
      logs,
      hostId,
      mutateAsync,
      setLogs,
      setIsFetchedUntilLogs,
    ],
  );

  useEffect(() => {
    setLogs([]);
    setIsFetchedUntilLogs(false);
    fetchLogs("init");
  }, [hostId]);

  useEffect(() => {
    if (!isTailing) {
      return;
    }
    const timer = setInterval(() => {
      fetchLogs("since");
    }, 5 * 1000);

    return () => {
      clearTimeout(timer);
    };
  }, [isTailing, fetchLogs]);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold">Logs</h3>
          <div className="flex items-center space-x-2">
            <Checkbox
              id="tail-logs"
              checked={isTailing}
              onCheckedChange={(checked) => setIsTailing(checked === true)}
            />
            <Label htmlFor="tail-logs">tail</Label>
          </div>
        </div>
      </CardHeader>
      <CardContent className="relative" style={{ height }}>
        <div ref={scrollAreaRef} className="absolute inset-0 overflow-y-auto">
          {!isFetchedUntilLogs && logs.length > 0 && (
            <div className="flex justify-center mb-2">
              <Button onClick={() => fetchLogs("until")} variant="ghost">
                以前のログを取得
              </Button>
            </div>
          )}
          {logs.length === 0 && (
            <div className="flex justify-center">
              <Button onClick={() => fetchLogs("init")} variant="ghost">
                ログを取得
              </Button>
            </div>
          )}
          {logs.map((log, i) => (
            <div key={i} className="font-mono text-sm">
              <span
                className={log.isError ? "text-destructive" : "text-foreground"}
              >
                {log.timestamp
                  ? new Date(
                      Number(log.timestamp.seconds) * 1000,
                    ).toLocaleTimeString("ja-JP") + " "
                  : ""}
                {log.body}
              </span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
