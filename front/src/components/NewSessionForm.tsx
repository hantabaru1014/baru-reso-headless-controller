import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  fetchWorldInfo,
  listHeadlessHost,
  startWorld,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { Button } from "./ui";
import { useNavigate } from "react-router";
import { AccessLevels } from "../constants";
import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { z } from "zod";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import {
  CheckboxField,
  RadioGroupField,
  TextareaField,
  TextField,
  SelectField,
} from "./base";

const sessionFormSchema = z
  .object({
    // protoにない独自フィールド
    hostId: z.string().min(1, "ホストを選択してください"),
    worldSource: z.enum(["url", "template"]),

    // WorldStartupParametersのフィールド順に対応
    name: z.string().min(1, "セッション名を入力してください"),
    customSessionId: z.string().optional(),
    description: z.string().optional(),
    tags: z.string().optional(),
    maxUsers: z.number().int().min(1, "最低1人以上の設定が必要です"),
    accessLevel: z.number().int().min(1).max(6),
    // loadWorld関連
    worldUrl: z.string().optional(),
    worldTemplate: z.enum(["grid", "platform", "blank"]),
    // 追加フィールド
    autoInviteUsernames: z.string().optional(),
    hideFromPublicListing: z.boolean(),
    defaultUserRoles: z
      .array(
        z.object({
          role: z.string(),
          userName: z.string(),
        }),
      )
      .optional(),
    awayKickMinutes: z.number(),
    idleRestartIntervalSeconds: z.number().int(),
    saveOnExit: z.boolean(),
    autoSaveIntervalSeconds: z.number().int(),
    autoSleep: z.boolean(),
    inviteRequestHandlerUsernames: z.string().optional(),
    forcePort: z.number().int().optional(),
    parentSessionIds: z.string().optional(),
    autoRecover: z.boolean().optional(),
    forcedRestartIntervalSeconds: z.number().int().optional(),
    useCustomJoinVerifier: z.boolean().optional(),
    mobileFriendly: z.boolean().optional(),
    overrideCorrespondingWorldId: z
      .string()
      .optional()
      .refine(
        (value) => {
          if (!value) return true; // 空文字列または未定義の場合はOK
          return /^[^/]+\/[^/]+$/.test(value); // ownerId/id 形式を検証
        },
        {
          message: "ownerId/id の形式で入力してください",
        },
      ),
    keepOriginalRoles: z.boolean().optional(),
    roleCloudVariable: z.string().optional(),
    allowUserCloudVariable: z.string().optional(),
    denyUserCloudVariable: z.string().optional(),
    requiredUserJoinCloudVariable: z.string().optional(),
    requiredUserJoinCloudVariableDenyMessage: z.string().optional(),
    autoInviteMessage: z.string().optional(),
  })
  .refine(
    (data) => {
      if (data.worldSource === "url") {
        return !!data.worldUrl;
      }
      return true;
    },
    {
      message: "URLを入力してください",
      path: ["worldUrl"],
    },
  );

// TODO: これ使ってるフォームはちゃんとしたのに作り変える！
const processCSV = (csv: string | undefined) =>
  csv
    ?.split(",")
    .map((n) => n.trim())
    .filter((n) => n) || [];

const processRecordId = (
  recordId: string | undefined,
): { id: string; ownerId: string } | undefined => {
  if (!recordId) return undefined;

  const parts = recordId.split("/");
  if (parts.length !== 2) return undefined;

  return {
    ownerId: parts[0],
    id: parts[1],
  };
};

export default function NewSessionForm() {
  const navigate = useNavigate();
  const { mutateAsync: mutateStart } = useMutation(startWorld);
  const { mutateAsync: mutateFetchInfo, isPending: isPendingFetchInfo } =
    useMutation(fetchWorldInfo);
  const { data: hostList } = useQuery(listHeadlessHost);

  const {
    control,
    handleSubmit,
    watch,
    setValue,
    formState: { errors },
  } = useForm<z.infer<typeof sessionFormSchema>>({
    resolver: zodResolver(sessionFormSchema),
    mode: "onBlur",
    defaultValues: {
      // 独自フィールド
      worldSource: "url",

      // WorldStartupParametersのフィールド
      worldTemplate: "grid",
      maxUsers: 15,
      accessLevel: 1,
      hideFromPublicListing: false,
      tags: "",
      autoInviteUsernames: "",
      defaultUserRoles: [],
      awayKickMinutes: -1,
      idleRestartIntervalSeconds: -1,
      saveOnExit: false,
      autoSaveIntervalSeconds: -1,
      autoSleep: false,
      inviteRequestHandlerUsernames: "",
      parentSessionIds: "",
      autoRecover: false,
      forcedRestartIntervalSeconds: -1,
      useCustomJoinVerifier: false,
      mobileFriendly: false,
      keepOriginalRoles: false,
    },
  });

  const hostId = watch("hostId");
  const worldSource = watch("worldSource");
  const worldUrl = watch("worldUrl");

  const handleFetchInfo = async () => {
    if (!hostId || !worldUrl) return;

    try {
      const data = await mutateFetchInfo({
        hostId,
        url: worldUrl,
      });
      setValue("name", data.name);
      setValue("description", data.description || "");
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "ワールド情報の取得に失敗しました",
      );
    }
  };

  const onSubmit = async (data: z.infer<typeof sessionFormSchema>) => {
    try {
      await mutateStart({
        hostId: data.hostId,
        parameters: {
          loadWorld:
            data.worldSource === "url"
              ? { case: "loadWorldUrl", value: data.worldUrl || "" }
              : { case: "loadWorldPresetName", value: data.worldTemplate },
          name: data.name,
          description: data.description || "",
          tags: processCSV(data.tags),
          maxUsers: data.maxUsers,
          accessLevel: data.accessLevel,
          customSessionId: data.customSessionId || "",
          autoInviteUsernames: processCSV(data.autoInviteUsernames),
          hideFromPublicListing: data.hideFromPublicListing,
          defaultUserRoles: data.defaultUserRoles || [],
          awayKickMinutes: data.awayKickMinutes,
          idleRestartIntervalSeconds: data.idleRestartIntervalSeconds,
          saveOnExit: data.saveOnExit,
          autoSaveIntervalSeconds: data.autoSaveIntervalSeconds,
          autoSleep: data.autoSleep,
          inviteRequestHandlerUsernames: processCSV(
            data.inviteRequestHandlerUsernames,
          ),
          forcePort: data.forcePort,
          parentSessionIds: processCSV(data.parentSessionIds),
          autoRecover: data.autoRecover,
          forcedRestartIntervalSeconds: data.forcedRestartIntervalSeconds,
          useCustomJoinVerifier: data.useCustomJoinVerifier,
          mobileFriendly: data.mobileFriendly,
          overrideCorrespondingWorldId: processRecordId(
            data.overrideCorrespondingWorldId,
          ),
          keepOriginalRoles: data.keepOriginalRoles,
          roleCloudVariable: data.roleCloudVariable,
          allowUserCloudVariable: data.allowUserCloudVariable,
          denyUserCloudVariable: data.denyUserCloudVariable,
          requiredUserJoinCloudVariable: data.requiredUserJoinCloudVariable,
          requiredUserJoinCloudVariableDenyMessage:
            data.requiredUserJoinCloudVariableDenyMessage,
          autoInviteMessage: data.autoInviteMessage,
        },
      });
      toast.success("セッションを開始しました");
      navigate("/sessions");
    } catch (e) {
      toast.error(`エラー: ${e instanceof Error ? e.message : e}`);
    }
  };

  return (
    <form className="space-y-4" onSubmit={handleSubmit(onSubmit)}>
      <Controller
        name="hostId"
        control={control}
        render={({ field }) => (
          <SelectField
            label="Host"
            options={
              hostList?.hosts
                .filter((host) => host.status === HeadlessHostStatus.RUNNING)
                .map((host) => ({
                  id: host.id,
                  label: `${host.name} (${host.id.slice(0, 6)})`,
                  value: host,
                })) ?? []
            }
            selectedId={field.value || ""}
            onChange={(option) => field.onChange(option.value?.id ?? "")}
            minWidth="7rem"
            error={errors.hostId?.message}
          />
        )}
      />
      <Controller
        name="worldSource"
        control={control}
        render={({ field }) => (
          <RadioGroupField
            label="ワールド指定方法"
            options={[
              { label: "レコードURLを指定", value: "url" },
              { label: "テンプレートを指定", value: "template" },
            ]}
            value={field.value}
            onValueChange={field.onChange}
            error={errors.worldSource?.message}
            className="flex flex-row space-x-4"
          />
        )}
      />
      {worldSource === "url" ? (
        <div className="grid grid-cols-12 gap-2">
          <div className="col-span-10">
            <Controller
              name="worldUrl"
              control={control}
              render={({ field }) => (
                <TextField
                  label="レコードURL"
                  error={errors.worldUrl?.message}
                  {...field}
                />
              )}
            />
          </div>
          <div className="col-span-2 flex items-end">
            <Button
              variant="outline"
              size="lg"
              onClick={handleFetchInfo}
              disabled={isPendingFetchInfo || !hostId || !worldUrl}
            >
              情報取得
            </Button>
          </div>
        </div>
      ) : (
        <Controller
          name="worldTemplate"
          control={control}
          render={({ field }) => (
            <SelectField
              label="ワールドテンプレート"
              options={[
                { id: "grid", label: "Grid" },
                { id: "platform", label: "Platform" },
                { id: "blank", label: "Blank" },
              ]}
              selectedId={field.value}
              onChange={(option) => field.onChange(option.id)}
              error={errors.worldTemplate?.message}
            />
          )}
        />
      )}
      <Controller
        name="name"
        control={control}
        render={({ field }) => (
          <TextField
            label="セッション名"
            error={errors.name?.message}
            {...field}
          />
        )}
      />
      <Controller
        name="description"
        control={control}
        render={({ field }) => (
          <TextareaField
            label="説明"
            error={errors.description?.message}
            {...field}
          />
        )}
      />
      <Controller
        name="tags"
        control={control}
        render={({ field }) => (
          <TextField
            label="タグ"
            error={errors.tags?.message}
            {...field}
            helperText="カンマ区切りで入力してください"
          />
        )}
      />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Controller
          name="maxUsers"
          control={control}
          render={({ field }) => (
            <TextField
              label="最大ユーザー数"
              type="number"
              error={errors.maxUsers?.message}
              {...field}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                const value =
                  e.target.value === "" ? "" : parseInt(e.target.value);
                field.onChange(value);
              }}
            />
          )}
        />

        <Controller
          name="accessLevel"
          control={control}
          render={({ field }) => (
            <SelectField
              label="アクセスレベル"
              options={AccessLevels.map((l) => l)}
              selectedId={`${field.value}`}
              onChange={(option) => field.onChange(option.value as number)}
              error={errors.accessLevel?.message}
            />
          )}
        />

        <Controller
          name="hideFromPublicListing"
          control={control}
          render={({ field }) => (
            <CheckboxField
              label="セッションリストから隠す"
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
      </div>
      <Controller
        name="customSessionId"
        control={control}
        render={({ field }) => (
          <TextField
            label="カスタムセッションID"
            error={errors.customSessionId?.message}
            {...field}
          />
        )}
      />

      <Controller
        name="parentSessionIds"
        control={control}
        render={({ field }) => (
          <TextField
            label="parentSessionIds"
            error={errors.parentSessionIds?.message}
            helperText="カンマ区切りで入力してください"
            {...field}
          />
        )}
      />

      <Controller
        name="overrideCorrespondingWorldId"
        control={control}
        render={({ field }) => (
          <TextField
            label="overrideCorrespondingWorldId"
            error={errors.overrideCorrespondingWorldId?.message}
            helperText="ownerId/id の形式で入力してください"
            {...field}
          />
        )}
      />

      <Controller
        name="awayKickMinutes"
        control={control}
        render={({ field }) => (
          <TextField
            label="AFKキック時間(分)"
            type="number"
            error={errors.awayKickMinutes?.message}
            helperText="-1で無効"
            {...field}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
              const value =
                e.target.value === "" ? "" : parseFloat(e.target.value);
              field.onChange(value);
            }}
          />
        )}
      />
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="autoSaveIntervalSeconds"
          control={control}
          render={({ field }) => (
            <TextField
              label="自動保存間隔(秒)"
              type="number"
              error={errors.autoSaveIntervalSeconds?.message}
              helperText="-1で無効"
              {...field}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                const value =
                  e.target.value === "" ? "" : parseInt(e.target.value);
                field.onChange(value);
              }}
            />
          )}
        />

        <Controller
          name="saveOnExit"
          control={control}
          render={({ field }) => (
            <CheckboxField
              label="セッション終了時に保存"
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {" "}
        <Controller
          name="idleRestartIntervalSeconds"
          control={control}
          render={({ field }) => (
            <TextField
              label="アイドル時の自動再起動間隔(秒)"
              type="number"
              error={errors.idleRestartIntervalSeconds?.message}
              helperText="-1で無効"
              {...field}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                const value =
                  e.target.value === "" ? "" : parseInt(e.target.value);
                field.onChange(value);
              }}
            />
          )}
        />
        <Controller
          name="forcedRestartIntervalSeconds"
          control={control}
          render={({ field }) => (
            <TextField
              label="forcedRestartInterval(秒)"
              type="number"
              error={errors.forcedRestartIntervalSeconds?.message}
              helperText="-1で無効"
              {...field}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                const value =
                  e.target.value === "" ? "" : parseInt(e.target.value);
                field.onChange(value);
              }}
            />
          )}
        />
        <Controller
          name="autoSleep"
          control={control}
          render={({ field }) => (
            <CheckboxField
              label="自動スリープ"
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
        <Controller
          name="autoRecover"
          control={control}
          render={({ field }) => (
            <CheckboxField
              label="autoRecover"
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="forcePort"
          control={control}
          render={({ field }) => (
            <TextField
              label="forcePort"
              type="number"
              error={errors.forcePort?.message}
              {...field}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                const value =
                  e.target.value === "" ? "" : parseInt(e.target.value);
                field.onChange(value);
              }}
            />
          )}
        />

        <Controller
          name="mobileFriendly"
          control={control}
          render={({ field }) => (
            <CheckboxField
              label="mobileFriendly"
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="keepOriginalRoles"
          control={control}
          render={({ field }) => (
            <CheckboxField
              label="keepOriginalRoles"
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />

        <Controller
          name="useCustomJoinVerifier"
          control={control}
          render={({ field }) => (
            <CheckboxField
              label="useCustomJoinVerifier"
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="autoInviteUsernames"
          control={control}
          render={({ field }) => (
            <TextareaField
              label="自動招待ユーザ"
              error={errors.autoInviteUsernames?.message}
              helperText="カンマ区切りで入力してください"
              {...field}
            />
          )}
        />

        <Controller
          name="autoInviteMessage"
          control={control}
          render={({ field }) => (
            <TextareaField
              label="招待メッセージ"
              error={errors.autoInviteMessage?.message}
              {...field}
            />
          )}
        />
      </div>

      <Controller
        name="roleCloudVariable"
        control={control}
        render={({ field }) => (
          <TextField
            label="roleCloudVariable"
            error={errors.roleCloudVariable?.message}
            {...field}
          />
        )}
      />

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="allowUserCloudVariable"
          control={control}
          render={({ field }) => (
            <TextField
              label="allowUserCloudVariable"
              error={errors.allowUserCloudVariable?.message}
              {...field}
            />
          )}
        />

        <Controller
          name="denyUserCloudVariable"
          control={control}
          render={({ field }) => (
            <TextField
              label="denyUserCloudVariable"
              error={errors.denyUserCloudVariable?.message}
              {...field}
            />
          )}
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="requiredUserJoinCloudVariable"
          control={control}
          render={({ field }) => (
            <TextField
              label="requiredUserJoinCloudVariable"
              error={errors.requiredUserJoinCloudVariable?.message}
              {...field}
            />
          )}
        />

        <Controller
          name="requiredUserJoinCloudVariableDenyMessage"
          control={control}
          render={({ field }) => (
            <TextField
              label="requiredUserJoinCloudVariableDenyMessage"
              error={errors.requiredUserJoinCloudVariableDenyMessage?.message}
              {...field}
            />
          )}
        />
      </div>

      <Button type="submit" disabled={Object.keys(errors).length > 0}>
        セッション開始
      </Button>
    </form>
  );
}
