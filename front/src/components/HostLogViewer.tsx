import { useMutation } from "@connectrpc/connect-query";
import {
  Button,
  Card,
  CardContent,
  CardHeader,
  Checkbox,
  FormControlLabel,
  Stack,
  Typography,
} from "@mui/material";
import { getHeadlessHostLogs } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  GetHeadlessHostLogsRequest,
  GetHeadlessHostLogsResponse_Log,
} from "../../pbgen/hdlctrl/v1/controller_pb";

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
        if (mode === "until") {
          setIsFetchedUntilLogs(true);
        }
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
        console.error(e);
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
    fetchLogs("init");
  }, []);

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
    <Card variant="outlined">
      <CardHeader
        title="Logs"
        action={
          <Stack direction="row">
            <FormControlLabel
              label="tail"
              control={
                <Checkbox
                  checked={isTailing}
                  onChange={(e) => setIsTailing(e.target.checked)}
                />
              }
            />
          </Stack>
        }
      />
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
          {!isFetchedUntilLogs && (
            <Stack justifyContent="center">
              <Button onClick={() => fetchLogs("until")} variant="text">
                以前のログを取得
              </Button>
            </Stack>
          )}
          {logs.map((log, i) => (
            <Stack key={i} direction="row">
              <Typography
                component="span"
                color={log.isError ? "error" : "textPrimary"}
              >
                {log.timestamp
                  ? new Date(
                      Number(log.timestamp.seconds) * 1000,
                    ).toLocaleTimeString("ja-JP") + " "
                  : ""}
                {log.body}
              </Typography>
            </Stack>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
