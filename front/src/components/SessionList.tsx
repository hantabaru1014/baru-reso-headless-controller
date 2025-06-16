import { Button } from "./ui/button";
import { useQuery } from "@connectrpc/connect-query";
import {
  listHeadlessHost,
  searchSessions,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useNavigate } from "react-router";
import { AccessLevels } from "../constants";
import RefetchButton from "./base/RefetchButton";
import { sessionStatusToLabel } from "../libs/sessionUtils";
import SelectField from "./base/SelectField";
import { ReactNode, useMemo, useState } from "react";
import { Session, SessionStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { ColumnDef } from "@tanstack/react-table";
import { DataTable } from "./base";

export default function SessionList() {
  const [filterState, setFilterState] = useState<{
    label: ReactNode;
    id: string;
    value: SessionStatus | undefined;
  }>({
    label: "全て",
    id: "ALL",
    value: undefined,
  });
  const [filterHostId, setFilterHostId] = useState("ALL");
  const { data: hosts } = useQuery(listHeadlessHost);
  const { data, isPending, refetch } = useQuery(searchSessions, {
    parameters: {
      status: filterState.value,
      hostId: filterHostId === "ALL" ? undefined : filterHostId,
    },
  });
  const navigate = useNavigate();

  const columns: ColumnDef<Session>[] = useMemo(() => {
    const hostNameMap =
      hosts?.hosts.reduce(
        (acc, host) => {
          acc[host.id] = host.name;
          return acc;
        },
        {} as Record<string, string>,
      ) || {};

    return [
      {
        accessorKey: "name",
        header: "セッション名",
      },
      {
        accessorKey: "hostId",
        header: "ホスト名",
        cell: ({ cell }) => hostNameMap[cell.getValue<string>()] || "不明",
      },
      {
        accessorKey: "status",
        header: "状態",
        cell: ({ cell }) =>
          sessionStatusToLabel(cell.getValue<SessionStatus>()),
      },
      {
        accessorKey: "currentState.accessLevel",
        header: "アクセスレベル",
        cell: ({ cell }) => {
          const accessLevel = cell.getValue<number>();
          const paramAccessLevel = cell.row.original.startupParameters
            ?.accessLevel as number;
          return (
            AccessLevels[accessLevel - 1]?.label ||
            AccessLevels[paramAccessLevel - 1]?.label ||
            "不明"
          );
        },
      },
      {
        accessorKey: "currentState.usersCount",
        header: "ユーザー数",
        cell: ({ row }) => {
          const currentState = row.original.currentState;
          const paramMaxUsers = row.original.startupParameters?.maxUsers;
          return currentState
            ? `${currentState.usersCount}/${currentState.maxUsers}`
            : paramMaxUsers
              ? `0/${paramMaxUsers || 0}`
              : "不明";
        },
      },
    ];
  }, [hosts?.hosts]);

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <div className="flex gap-2">
          <SelectField
            label="状態"
            options={[
              { label: "全て", id: "ALL", value: undefined },
              { label: "実行中", id: "RUNNING", value: SessionStatus.RUNNING },
              { label: "終了済み", id: "ENDED", value: SessionStatus.ENDED },
            ]}
            onChange={(o) =>
              setFilterState({
                label: o.label,
                id: o.id,
                value: o.value,
              })
            }
            selectedId={filterState.id}
          />
          <SelectField
            label="ホスト"
            options={[{ label: "全て", id: "ALL" }].concat(
              hosts?.hosts.map((host) => ({
                label: host.name,
                id: host.id,
              })) || [],
            )}
            onChange={(o) => setFilterHostId(o.id)}
            selectedId={filterHostId}
          />
        </div>
        <div className="flex gap-2">
          <RefetchButton refetch={refetch} />
          <Button onClick={() => navigate("/sessions/new")}>
            新規セッション
          </Button>
        </div>
      </div>
      <DataTable
        columns={columns}
        data={data?.sessions || []}
        isLoading={isPending}
        onClickRow={(row) => navigate(`/sessions/${row.id}`)}
      />
    </div>
  );
}
