import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  fetchWorldInfo,
  listHeadlessHost,
  startWorld,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import {
  Button,
  Checkbox,
  Label,
  Input,
  Textarea,
  RadioGroup,
  RadioGroupItem,
} from "./ui";
import SelectField from "./base/SelectField";
import { useNavigate } from "react-router";
import { AccessLevels } from "../constants";
import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { z } from "zod";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";

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
            error={!!errors.hostId}
            helperText={errors.hostId?.message}
          />
        )}
      />
      <Controller
        name="worldSource"
        control={control}
        render={({ field }) => (
          <div className="space-y-2">
            <Label htmlFor="session-form-use-world-url">ワールド指定方法</Label>
            <RadioGroup
              value={field.value}
              onValueChange={field.onChange}
              className="flex flex-row space-x-4"
            >
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="url" id="url" />
                <Label htmlFor="url">レコードURLを指定</Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="template" id="template" />
                <Label htmlFor="template">テンプレートを指定</Label>
              </div>
            </RadioGroup>
          </div>
        )}
      />
      {worldSource === "url" ? (
        <div className="grid grid-cols-12 gap-2">
          <div className="col-span-10">
            <Controller
              name="worldUrl"
              control={control}
              render={({ field }) => (
                <div className="space-y-2">
                  <Label htmlFor="worldUrl">レコードURL</Label>
                  <Input
                    id="worldUrl"
                    {...field}
                    className={errors.worldUrl ? "border-red-500" : ""}
                  />
                  {errors.worldUrl && (
                    <p className="text-sm text-red-500">
                      {errors.worldUrl.message}
                    </p>
                  )}
                </div>
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
              error={!!errors.worldTemplate}
              helperText={errors.worldTemplate?.message}
            />
          )}
        />
      )}
      <Controller
        name="name"
        control={control}
        render={({ field }) => (
          <div className="space-y-2">
            <Label htmlFor="name">セッション名</Label>
            <Input
              id="name"
              {...field}
              className={errors.name ? "border-red-500" : ""}
            />
            {errors.name && (
              <p className="text-sm text-red-500">{errors.name.message}</p>
            )}
          </div>
        )}
      />
      <Controller
        name="description"
        control={control}
        render={({ field }) => (
          <div className="space-y-2">
            <Label htmlFor="description">説明</Label>
            <Textarea
              id="description"
              {...field}
              className={errors.description ? "border-red-500" : ""}
            />
            {errors.description && (
              <p className="text-sm text-red-500">
                {errors.description.message}
              </p>
            )}
          </div>
        )}
      />
      <Controller
        name="tags"
        control={control}
        render={({ field }) => (
          <div className="space-y-2">
            <Label htmlFor="tags">タグ</Label>
            <Input
              id="tags"
              {...field}
              className={errors.tags ? "border-red-500" : ""}
            />
            {errors.tags && (
              <p className="text-sm text-red-500">{errors.tags.message}</p>
            )}
            <p className="text-sm text-gray-500">
              カンマ区切りで入力してください
            </p>
          </div>
        )}
      />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Controller
          name="maxUsers"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="maxUsers">最大ユーザー数</Label>
              <Input
                id="maxUsers"
                type="number"
                {...field}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                  const value =
                    e.target.value === "" ? "" : parseInt(e.target.value);
                  field.onChange(value);
                }}
                className={errors.maxUsers ? "border-red-500" : ""}
              />
              {errors.maxUsers && (
                <p className="text-sm text-red-500">
                  {errors.maxUsers.message}
                </p>
              )}
            </div>
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
              error={!!errors.accessLevel}
              helperText={errors.accessLevel?.message}
            />
          )}
        />

        <Controller
          name="hideFromPublicListing"
          control={control}
          render={({ field }) => (
            <div className="flex items-center space-x-2 pt-6">
              <Checkbox
                id="hideFromPublicListing"
                checked={field.value}
                onCheckedChange={field.onChange}
              />
              <Label htmlFor="hideFromPublicListing">
                セッションリストから隠す
              </Label>
            </div>
          )}
        />
      </div>
      <Controller
        name="customSessionId"
        control={control}
        render={({ field }) => (
          <div className="space-y-2">
            <Label htmlFor="customSessionId">カスタムセッションID</Label>
            <Input
              id="customSessionId"
              {...field}
              className={errors.customSessionId ? "border-red-500" : ""}
            />
            {errors.customSessionId && (
              <p className="text-sm text-red-500">
                {errors.customSessionId.message}
              </p>
            )}
          </div>
        )}
      />

      <Controller
        name="parentSessionIds"
        control={control}
        render={({ field }) => (
          <div className="space-y-2">
            <Label htmlFor="parentSessionIds">parentSessionIds</Label>
            <Input
              id="parentSessionIds"
              {...field}
              className={errors.parentSessionIds ? "border-red-500" : ""}
            />
            {errors.parentSessionIds && (
              <p className="text-sm text-red-500">
                {errors.parentSessionIds.message}
              </p>
            )}
            <p className="text-sm text-gray-500">
              カンマ区切りで入力してください
            </p>
          </div>
        )}
      />

      <Controller
        name="overrideCorrespondingWorldId"
        control={control}
        render={({ field }) => (
          <div className="space-y-2">
            <Label htmlFor="overrideCorrespondingWorldId">
              overrideCorrespondingWorldId
            </Label>
            <Input
              id="overrideCorrespondingWorldId"
              {...field}
              className={
                errors.overrideCorrespondingWorldId ? "border-red-500" : ""
              }
            />
            {errors.overrideCorrespondingWorldId && (
              <p className="text-sm text-red-500">
                {errors.overrideCorrespondingWorldId.message}
              </p>
            )}
            <p className="text-sm text-gray-500">
              ownerId/id の形式で入力してください
            </p>
          </div>
        )}
      />

      <Controller
        name="awayKickMinutes"
        control={control}
        render={({ field }) => (
          <div className="space-y-2">
            <Label htmlFor="awayKickMinutes">AFKキック時間(分)</Label>
            <Input
              id="awayKickMinutes"
              type="number"
              {...field}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                const value =
                  e.target.value === "" ? "" : parseFloat(e.target.value);
                field.onChange(value);
              }}
              className={errors.awayKickMinutes ? "border-red-500" : ""}
            />
            {errors.awayKickMinutes && (
              <p className="text-sm text-red-500">
                {errors.awayKickMinutes.message}
              </p>
            )}
            <p className="text-sm text-gray-500">-1で無効</p>
          </div>
        )}
      />
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="autoSaveIntervalSeconds"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="autoSaveIntervalSeconds">自動保存間隔(秒)</Label>
              <Input
                id="autoSaveIntervalSeconds"
                type="number"
                {...field}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                  const value =
                    e.target.value === "" ? "" : parseInt(e.target.value);
                  field.onChange(value);
                }}
                className={
                  errors.autoSaveIntervalSeconds ? "border-red-500" : ""
                }
              />
              {errors.autoSaveIntervalSeconds && (
                <p className="text-sm text-red-500">
                  {errors.autoSaveIntervalSeconds.message}
                </p>
              )}
              <p className="text-sm text-gray-500">-1で無効</p>
            </div>
          )}
        />

        <Controller
          name="saveOnExit"
          control={control}
          render={({ field }) => (
            <div className="flex items-center space-x-2 pt-6">
              <Checkbox
                id="saveOnExit"
                checked={field.value}
                onCheckedChange={field.onChange}
              />
              <Label htmlFor="saveOnExit">セッション終了時に保存</Label>
            </div>
          )}
        />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Controller
          name="idleRestartIntervalSeconds"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="idleRestartIntervalSeconds">
                アイドル時の自動再起動間隔(秒)
              </Label>
              <Input
                id="idleRestartIntervalSeconds"
                type="number"
                {...field}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                  const value =
                    e.target.value === "" ? "" : parseInt(e.target.value);
                  field.onChange(value);
                }}
                className={
                  errors.idleRestartIntervalSeconds ? "border-red-500" : ""
                }
              />
              {errors.idleRestartIntervalSeconds && (
                <p className="text-sm text-red-500">
                  {errors.idleRestartIntervalSeconds.message}
                </p>
              )}
              <p className="text-sm text-gray-500">-1で無効</p>
            </div>
          )}
        />

        <Controller
          name="forcedRestartIntervalSeconds"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="forcedRestartIntervalSeconds">
                forcedRestartInterval(秒)
              </Label>
              <Input
                id="forcedRestartIntervalSeconds"
                type="number"
                {...field}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                  const value =
                    e.target.value === "" ? "" : parseInt(e.target.value);
                  field.onChange(value);
                }}
                className={
                  errors.forcedRestartIntervalSeconds ? "border-red-500" : ""
                }
              />
              {errors.forcedRestartIntervalSeconds && (
                <p className="text-sm text-red-500">
                  {errors.forcedRestartIntervalSeconds.message}
                </p>
              )}
              <p className="text-sm text-gray-500">-1で無効</p>
            </div>
          )}
        />

        <Controller
          name="autoSleep"
          control={control}
          render={({ field }) => (
            <div className="flex items-center space-x-2 pt-6">
              <Checkbox
                id="autoSleep"
                checked={field.value}
                onCheckedChange={field.onChange}
              />
              <Label htmlFor="autoSleep">自動スリープ</Label>
            </div>
          )}
        />

        <Controller
          name="autoRecover"
          control={control}
          render={({ field }) => (
            <div className="flex items-center space-x-2 pt-6">
              <Checkbox
                id="autoRecover"
                checked={field.value}
                onCheckedChange={field.onChange}
              />
              <Label htmlFor="autoRecover">autoRecover</Label>
            </div>
          )}
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="forcePort"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="forcePort">forcePort</Label>
              <Input
                id="forcePort"
                type="number"
                {...field}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                  const value =
                    e.target.value === "" ? "" : parseInt(e.target.value);
                  field.onChange(value);
                }}
                className={errors.forcePort ? "border-red-500" : ""}
              />
              {errors.forcePort && (
                <p className="text-sm text-red-500">
                  {errors.forcePort.message}
                </p>
              )}
            </div>
          )}
        />

        <Controller
          name="mobileFriendly"
          control={control}
          render={({ field }) => (
            <div className="flex items-center space-x-2 pt-6">
              <Checkbox
                id="mobileFriendly"
                checked={field.value}
                onCheckedChange={field.onChange}
              />
              <Label htmlFor="mobileFriendly">mobileFriendly</Label>
            </div>
          )}
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="keepOriginalRoles"
          control={control}
          render={({ field }) => (
            <div className="flex items-center space-x-2">
              <Checkbox
                id="keepOriginalRoles"
                checked={field.value}
                onCheckedChange={field.onChange}
              />
              <Label htmlFor="keepOriginalRoles">keepOriginalRoles</Label>
            </div>
          )}
        />

        <Controller
          name="useCustomJoinVerifier"
          control={control}
          render={({ field }) => (
            <div className="flex items-center space-x-2">
              <Checkbox
                id="useCustomJoinVerifier"
                checked={field.value}
                onCheckedChange={field.onChange}
              />
              <Label htmlFor="useCustomJoinVerifier">
                useCustomJoinVerifier
              </Label>
            </div>
          )}
        />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="autoInviteUsernames"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="autoInviteUsernames">自動招待ユーザ</Label>
              <Textarea
                id="autoInviteUsernames"
                {...field}
                className={errors.autoInviteUsernames ? "border-red-500" : ""}
              />
              {errors.autoInviteUsernames && (
                <p className="text-sm text-red-500">
                  {errors.autoInviteUsernames.message}
                </p>
              )}
              <p className="text-sm text-gray-500">
                カンマ区切りで入力してください
              </p>
            </div>
          )}
        />

        <Controller
          name="autoInviteMessage"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="autoInviteMessage">招待メッセージ</Label>
              <Textarea
                id="autoInviteMessage"
                {...field}
                className={errors.autoInviteMessage ? "border-red-500" : ""}
              />
              {errors.autoInviteMessage && (
                <p className="text-sm text-red-500">
                  {errors.autoInviteMessage.message}
                </p>
              )}
            </div>
          )}
        />
      </div>

      <Controller
        name="roleCloudVariable"
        control={control}
        render={({ field }) => (
          <div className="space-y-2">
            <Label htmlFor="roleCloudVariable">roleCloudVariable</Label>
            <Input
              id="roleCloudVariable"
              {...field}
              className={errors.roleCloudVariable ? "border-red-500" : ""}
            />
            {errors.roleCloudVariable && (
              <p className="text-sm text-red-500">
                {errors.roleCloudVariable.message}
              </p>
            )}
          </div>
        )}
      />

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="allowUserCloudVariable"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="allowUserCloudVariable">
                allowUserCloudVariable
              </Label>
              <Input
                id="allowUserCloudVariable"
                {...field}
                className={
                  errors.allowUserCloudVariable ? "border-red-500" : ""
                }
              />
              {errors.allowUserCloudVariable && (
                <p className="text-sm text-red-500">
                  {errors.allowUserCloudVariable.message}
                </p>
              )}
            </div>
          )}
        />

        <Controller
          name="denyUserCloudVariable"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="denyUserCloudVariable">
                denyUserCloudVariable
              </Label>
              <Input
                id="denyUserCloudVariable"
                {...field}
                className={errors.denyUserCloudVariable ? "border-red-500" : ""}
              />
              {errors.denyUserCloudVariable && (
                <p className="text-sm text-red-500">
                  {errors.denyUserCloudVariable.message}
                </p>
              )}
            </div>
          )}
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Controller
          name="requiredUserJoinCloudVariable"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="requiredUserJoinCloudVariable">
                requiredUserJoinCloudVariable
              </Label>
              <Input
                id="requiredUserJoinCloudVariable"
                {...field}
                className={
                  errors.requiredUserJoinCloudVariable ? "border-red-500" : ""
                }
              />
              {errors.requiredUserJoinCloudVariable && (
                <p className="text-sm text-red-500">
                  {errors.requiredUserJoinCloudVariable.message}
                </p>
              )}
            </div>
          )}
        />

        <Controller
          name="requiredUserJoinCloudVariableDenyMessage"
          control={control}
          render={({ field }) => (
            <div className="space-y-2">
              <Label htmlFor="requiredUserJoinCloudVariableDenyMessage">
                requiredUserJoinCloudVariableDenyMessage
              </Label>
              <Input
                id="requiredUserJoinCloudVariableDenyMessage"
                {...field}
                className={
                  errors.requiredUserJoinCloudVariableDenyMessage
                    ? "border-red-500"
                    : ""
                }
              />
              {errors.requiredUserJoinCloudVariableDenyMessage && (
                <p className="text-sm text-red-500">
                  {errors.requiredUserJoinCloudVariableDenyMessage.message}
                </p>
              )}
            </div>
          )}
        />
      </div>

      <Button type="submit" disabled={Object.keys(errors).length > 0}>
        セッション開始
      </Button>
    </form>
  );
}
