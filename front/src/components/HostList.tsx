import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  buildResoniteImage,
  listHeadlessAccounts,
  listHeadlessHost,
  listResoniteVersions,
  startHeadlessHost,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { ResoniteVersionBuildStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { ColumnDef } from "@tanstack/react-table";
import { keepPreviousData } from "@tanstack/react-query";
import { usePaginationState } from "../hooks/usePaginationState";
import { useDefaultGroupId } from "../hooks/useDefaultGroupId";
import { useAtomValue } from "jotai";
import { currentGroupIdAtom } from "../atoms/currentGroupAtom";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogClose,
} from "./ui/dialog";
import { Button } from "./ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "./ui/collapsible";
import { ChevronDown } from "lucide-react";
import { useNavigate } from "react-router";
import { hostStatusToLabel } from "../libs/hostUtils";
import { RefetchButton } from "./base/RefetchButton";
import { useEffect, useMemo, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { SelectField } from "./base/SelectField";
import {
  HeadlessHost,
  HeadlessHostAutoUpdatePolicy,
} from "../../pbgen/hdlctrl/v1/controller_pb";
import { DataTable } from "./base/DataTable";
import { toast } from "sonner";
import { CheckboxField, TextField } from "./base";
import { ResoniteUserIcon } from "./ResoniteUserIcon";
import { GroupSelectField } from "./GroupSelectField";
import { PermissionGuardedButton } from "./base/PermissionGuardedButton";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";

const makeNewHostFormSchema = (t: TFunction) =>
  z.object({
    name: z.string().min(1, t("hostList.nameRequired")),
    universeId: z.string().optional(),
    usernameOverride: z.string().optional(),
    tag: z.string().min(1, t("hostList.versionRequired")),
    accountId: z.string().min(1, t("hostList.hostUserRequired")),
    groupId: z.string().min(1, t("hostList.groupRequired")),
    autoUpdate: z.boolean(),
  });
type NewHostFormData = z.infer<ReturnType<typeof makeNewHostFormSchema>>;

function NewHostDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose?: () => void;
}) {
  const { t } = useTranslation();
  const { data: accounts } = useQuery(listHeadlessAccounts);
  const { data: resoVersions } = useQuery(listResoniteVersions, {});
  const { mutateAsync: mutateStartHost, isPending } =
    useMutation(startHeadlessHost);
  const { mutateAsync: mutateBuildImage } = useMutation(buildResoniteImage);
  const [isAdvancedOpen, setIsAdvancedOpen] = useState(false);
  const defaultGroupId = useDefaultGroupId(PERMISSION_KEYS.HOST_WRITE);
  const schema = useMemo(() => makeNewHostFormSchema(t), [t]);

  const {
    control,
    handleSubmit,
    reset,
    watch,
    setValue,
    getValues,
    formState: { errors },
  } = useForm<NewHostFormData>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      universeId: "",
      usernameOverride: "",
      tag: "latestRelease",
      accountId: "",
      groupId: "",
      autoUpdate: false,
    },
  });

  const selectedGroupId = watch("groupId");

  // 非同期に解決される default group をユーザーが触っていなければ反映する.
  useEffect(() => {
    if (defaultGroupId && !getValues("groupId")) {
      setValue("groupId", defaultGroupId);
    }
  }, [defaultGroupId, getValues, setValue]);

  const onSubmit = async (data: NewHostFormData) => {
    const startReq = {
      name: data.name,
      headlessAccountId: data.accountId,
      imageTag: data.tag,
      startupConfig: {
        universeId: data.universeId || undefined,
        usernameOverride: data.usernameOverride || undefined,
      },
      autoUpdatePolicy: data.autoUpdate
        ? HeadlessHostAutoUpdatePolicy.USERS_EMPTY
        : HeadlessHostAutoUpdatePolicy.NEVER,
      groupId: data.groupId || undefined,
    };
    try {
      // "manifest:<branch>:<id>" 形式なら未 built のバージョンを指定している.
      // BUILD_IMAGE を chain 経由で enqueue する.
      if (data.tag.startsWith("manifest:")) {
        const parts = data.tag.split(":");
        const branch = parts[1];
        const manifestId = parts.slice(2).join(":");
        await mutateBuildImage({
          manifestId,
          branch,
          followUp: {
            case: "thenStartHost",
            value: { ...startReq, imageTag: undefined },
          },
        });
        toast.success(t("hostList.buildAndStartAccepted"));
      } else {
        await mutateStartHost(startReq);
        toast.success(t("hostList.startAccepted"));
      }
      onClose?.();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("hostList.startFailed"));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(open) => {
        if (!open) {
          onClose?.();
          reset();
        }
      }}
    >
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{t("hostList.startHeadless")}</DialogTitle>
        </DialogHeader>
        <form
          id="new-host-form"
          onSubmit={handleSubmit(onSubmit)}
          className="space-y-4"
        >
          <Controller
            name="name"
            control={control}
            render={({ field }) => (
              <TextField
                label={t("hostList.hostName")}
                {...field}
                error={errors.name?.message}
              />
            )}
          />
          <Controller
            name="tag"
            control={control}
            render={({ field }) => {
              const rawVersions = (resoVersions?.versions ?? []).filter(
                (v) =>
                  (v.branch === "headless" || v.branch === "prerelease") &&
                  !!v.gameVersion,
              );
              // 同一 gameVersion が複数の manifest で存在するケース (prerelease で
              // 再ビルドされた版など) を区別するため、重複ラベルには manifestId の
              // 先頭 8 桁を付与する.
              const labelCounts = rawVersions.reduce<Record<string, number>>(
                (acc, v) => {
                  const key = `${v.gameVersion}|${v.branch}`;
                  acc[key] = (acc[key] ?? 0) + 1;
                  return acc;
                },
                {},
              );
              const dynamicVersionOptions = rawVersions.map((v) => {
                const built =
                  v.buildStatus === ResoniteVersionBuildStatus.BUILT &&
                  v.imageTag;
                const key = `${v.gameVersion}|${v.branch}`;
                const disambig =
                  (labelCounts[key] ?? 0) > 1
                    ? ` [#${v.manifestId.slice(0, 8)}]`
                    : "";
                const label = `${v.gameVersion} (${v.branch})${disambig}${
                  built ? "" : ` ${t("hostList.needsBuild")}`
                }`;
                return {
                  id: built
                    ? (v.imageTag as string)
                    : `manifest:${v.branch}:${v.manifestId}`,
                  label,
                  manifestId: v.manifestId,
                  branch: v.branch,
                  built,
                };
              });
              const options = [
                { id: "latestRelease", label: t("hostList.latestRelease") },
                {
                  id: "latestPreRelease",
                  label: t("hostList.latestPreRelease"),
                },
                ...dynamicVersionOptions.map((o) => ({
                  id: o.id,
                  label: o.label,
                })),
              ];
              return (
                <SelectField
                  label={t("hostList.version")}
                  options={options}
                  selectedId={field.value}
                  onChange={(option) => field.onChange(option.id)}
                  error={errors.tag?.message}
                  helperText={t("hostList.versionHelper")}
                />
              );
            }}
          />
          <Controller
            name="groupId"
            control={control}
            render={({ field }) => (
              <GroupSelectField
                value={field.value}
                onChange={field.onChange}
                requiredPermission={PERMISSION_KEYS.HOST_WRITE}
                helperText={t("hostList.groupHelper")}
                error={errors.groupId?.message}
              />
            )}
          />
          <Controller
            name="accountId"
            control={control}
            render={({ field }) => (
              <SelectField
                label={t("hostList.hostUser")}
                helperText={t("hostList.hostUserHelper")}
                options={
                  accounts?.accounts
                    .filter(
                      (account) =>
                        !selectedGroupId || account.groupId === selectedGroupId,
                    )
                    .map((account) => ({
                      id: account.userId,
                      label: (
                        <span className="flex items-center gap-2">
                          <ResoniteUserIcon
                            iconUrl={account.iconUrl}
                            alt={t("hostList.userIconAlt", {
                              name: account.userName,
                            })}
                          />
                          <span className="text-sm font-medium">
                            {account.userName}
                          </span>
                        </span>
                      ),
                    })) ?? []
                }
                selectedId={field.value}
                onChange={(option) => field.onChange(option.id)}
                error={errors.accountId?.message}
              />
            )}
          />
          <Controller
            name="autoUpdate"
            control={control}
            render={({ field }) => (
              <CheckboxField
                label={t("hostList.autoUpdate")}
                helperText={t("hostList.autoUpdateHelper")}
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(checked === true)}
              />
            )}
          />
          <Collapsible open={isAdvancedOpen} onOpenChange={setIsAdvancedOpen}>
            <CollapsibleTrigger className="flex w-full items-center justify-between py-2">
              <span className="text-sm font-medium">
                {t("hostList.advancedSettings")}
              </span>
              <ChevronDown className="h-4 w-4" />
            </CollapsibleTrigger>
            <CollapsibleContent className="space-y-2">
              <Controller
                name="universeId"
                control={control}
                render={({ field }) => (
                  <TextField
                    label="Universe ID"
                    {...field}
                    error={errors.universeId?.message}
                  />
                )}
              />
              <Controller
                name="usernameOverride"
                control={control}
                render={({ field }) => (
                  <TextField
                    label="Username Override"
                    {...field}
                    error={errors.usernameOverride?.message}
                  />
                )}
              />
            </CollapsibleContent>
          </Collapsible>
        </form>
        <DialogFooter>
          <Button type="submit" form="new-host-form" disabled={isPending}>
            {t("common.start")}
          </Button>
          <DialogClose asChild>
            <Button variant="outline">{t("common.cancel")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default function HostList() {
  const { t } = useTranslation();
  const { pageIndex, pageSize, setPageIndex, setPageSize } = usePaginationState(
    { defaultPageSize: 20 },
  );
  const currentGroupId = useAtomValue(currentGroupIdAtom);
  const { data, isPending, refetch } = useQuery(
    listHeadlessHost,
    {
      page: { pageIndex, pageSize },
      groupId: currentGroupId ?? undefined,
    },
    { placeholderData: keepPreviousData },
  );
  const navigate = useNavigate();
  const [isNewHostDialogOpen, setIsNewHostDialogOpen] = useState(false);
  const { groupsWithPermission } = usePermissions();
  const canStartHost =
    groupsWithPermission(PERMISSION_KEYS.HOST_WRITE).length > 0;

  const columns: ColumnDef<HeadlessHost>[] = useMemo(
    () => [
      {
        accessorKey: "id",
        header: "ID",
        cell: ({ cell }) => cell.getValue<string>().slice(0, 16),
      },
      {
        accessorKey: "name",
        header: t("common.name"),
      },
      {
        accessorKey: "accountName",
        header: t("hostList.accountName"),
      },
      {
        accessorKey: "status",
        header: t("common.status"),
        cell: ({ row }) => hostStatusToLabel(row.original.status),
      },
      {
        accessorKey: "resoniteVersion",
        header: t("hostList.version"),
        cell: ({ row }) =>
          row.original.resoniteVersion
            ? `${row.original.resoniteVersion} (v${row.original.appVersion})`
            : t("common.unknown"),
      },
      {
        accessorKey: "fps",
        header: "fps",
        cell: ({ row }) =>
          row.original.fps ? Math.floor(row.original.fps * 10) / 10 : "N/A",
      },
    ],
    [t],
  );

  return (
    <div className="space-y-4">
      <div className="flex justify-end gap-2">
        <RefetchButton refetch={refetch} />
        <PermissionGuardedButton
          allowed={canStartHost}
          disabledReason={t("hostList.noStartPermission")}
          onClick={() => setIsNewHostDialogOpen(true)}
        >
          {t("hostList.startHeadless")}
        </PermissionGuardedButton>
      </div>
      <DataTable
        columns={columns}
        data={data?.hosts ?? []}
        isLoading={isPending}
        onClickRow={(row) => navigate(`/hosts/${row.id}`)}
        pagination={{
          pageIndex,
          pageSize,
          totalCount: data?.page?.totalCount ?? 0,
          onPageIndexChange: setPageIndex,
          onPageSizeChange: setPageSize,
        }}
      />
      <NewHostDialog
        open={isNewHostDialogOpen}
        onClose={() => {
          setIsNewHostDialogOpen(false);
          refetch();
        }}
      />
    </div>
  );
}
