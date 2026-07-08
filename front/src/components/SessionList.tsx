import { useQuery } from "@connectrpc/connect-query";
import {
  listHeadlessHost,
  searchSessions,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useNavigate } from "react-router";
import { AccessLevels } from "../constants";
import { RefetchButton } from "./base/RefetchButton";
import { sessionStatusToLabel } from "../libs/sessionUtils";
import { SelectField } from "./base/SelectField";
import { ReactNode, useMemo, useState } from "react";
import { Session, SessionStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { ColumnDef } from "@tanstack/react-table";
import { DataTable } from "./base";
import { RichText } from "./base/RichText";
import { keepPreviousData } from "@tanstack/react-query";
import { usePaginationState } from "../hooks/usePaginationState";
import { useAtomValue } from "jotai";
import { currentGroupIdAtom } from "../atoms/currentGroupAtom";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { useTranslation } from "react-i18next";

export default function SessionList() {
  const { t } = useTranslation();
  const [filterState, setFilterState] = useState<{
    label: ReactNode;
    id: string;
    value: SessionStatus | undefined;
  }>({
    label: t("sessionList.filterAll"),
    id: "ALL",
    value: undefined,
  });
  const [filterHostId, setFilterHostId] = useState("ALL");
  const currentGroupId = useAtomValue(currentGroupIdAtom);
  const { data: hosts } = useQuery(listHeadlessHost, {
    groupId: currentGroupId ?? undefined,
  });
  const { pageIndex, pageSize, setPageIndex, setPageSize } = usePaginationState(
    { defaultPageSize: 20 },
  );
  const { data, isPending, refetch } = useQuery(
    searchSessions,
    {
      parameters: {
        status: filterState.value,
        hostId: filterHostId === "ALL" ? undefined : filterHostId,
        groupId: currentGroupId ?? undefined,
      },
      page: { pageIndex, pageSize },
    },
    { placeholderData: keepPreviousData },
  );
  const navigate = useNavigate();
  const { groupsWithPermission } = usePermissions();
  const canCreate =
    groupsWithPermission(PERMISSION_KEYS.SESSION_WRITE).length > 0;

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
        header: t("sessionList.columnName"),
        cell: ({ cell }) => (
          <RichText text={cell.getValue<string>()} ignoreLayoutTags />
        ),
      },
      {
        accessorKey: "hostId",
        header: t("sessionList.columnHost"),
        cell: ({ cell }) =>
          hostNameMap[cell.getValue<string>()] || t("sessionList.unknown"),
      },
      {
        accessorKey: "status",
        header: t("sessionList.columnStatus"),
        cell: ({ cell }) =>
          sessionStatusToLabel(cell.getValue<SessionStatus>()),
      },
      {
        accessorKey: "currentState.accessLevel",
        header: t("sessionList.columnAccessLevel"),
        cell: ({ cell }) => {
          const accessLevel = cell.getValue<number>();
          const paramAccessLevel = cell.row.original.startupParameters
            ?.accessLevel as number;
          const labelKey =
            AccessLevels[accessLevel - 1]?.labelKey ||
            AccessLevels[paramAccessLevel - 1]?.labelKey;
          return labelKey ? t(labelKey) : t("sessionList.unknown");
        },
      },
      {
        accessorKey: "currentState.usersCount",
        header: t("sessionList.columnUsersCount"),
        cell: ({ row }) => {
          const currentState = row.original.currentState;
          const paramMaxUsers = row.original.startupParameters?.maxUsers;
          return currentState
            ? `${currentState.usersCount}/${currentState.maxUsers}`
            : paramMaxUsers
              ? `0/${paramMaxUsers || 0}`
              : t("sessionList.unknown");
        },
      },
    ];
  }, [hosts?.hosts, t]);

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <div className="flex gap-2">
          <SelectField
            label={t("sessionList.columnStatus")}
            options={[
              {
                label: t("sessionList.filterAll"),
                id: "ALL",
                value: undefined,
              },
              {
                label: t("sessionList.filterRunning"),
                id: "RUNNING",
                value: SessionStatus.RUNNING,
              },
              {
                label: t("sessionList.filterEnded"),
                id: "ENDED",
                value: SessionStatus.ENDED,
              },
            ]}
            onChange={(o) => {
              setFilterState({
                label: o.label,
                id: o.id,
                value: o.value,
              });
              setPageIndex(0);
            }}
            selectedId={filterState.id}
          />
          <SelectField
            label={t("sessionList.hostLabel")}
            options={[{ label: t("sessionList.filterAll"), id: "ALL" }].concat(
              hosts?.hosts.map((host) => ({
                label: host.name,
                id: host.id,
              })) || [],
            )}
            onChange={(o) => {
              setFilterHostId(o.id);
              setPageIndex(0);
            }}
            selectedId={filterHostId}
          />
        </div>
        <div className="flex gap-2">
          <RefetchButton refetch={refetch} />
          <PermissionGuardedButton
            allowed={canCreate}
            disabledReason={t("sessionList.noPermissionToCreate")}
            onClick={() => navigate("/sessions/new")}
          >
            {t("sessionList.newSession")}
          </PermissionGuardedButton>
        </div>
      </div>
      <DataTable
        columns={columns}
        data={data?.sessions || []}
        isLoading={isPending}
        onClickRow={(row) => navigate(`/sessions/${row.id}`)}
        pagination={{
          pageIndex,
          pageSize,
          totalCount: data?.page?.totalCount ?? 0,
          onPageIndexChange: setPageIndex,
          onPageSizeChange: setPageSize,
        }}
      />
    </div>
  );
}
