import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
  fetchWorldInfo,
  listHeadlessHost,
  startWorld,
} from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useNotifications } from "@toolpad/core/useNotifications";
import {
  Button,
  Checkbox,
  FormControl,
  FormControlLabel,
  FormLabel,
  Grid2,
  Radio,
  RadioGroup,
  Stack,
  TextField,
} from "@mui/material";
import SelectField from "./base/SelectField";
import { useNavigate } from "react-router";
import { AccessLevels } from "../constants";
import { HeadlessHostStatus } from "../../pbgen/hdlctrl/v1/controller_pb";
import { z } from "zod";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

const sessionFormSchema = z
  .object({
    hostId: z.string().min(1, "ホストを選択してください"),
    worldSource: z.enum(["url", "template"]),
    worldUrl: z.string().optional(),
    worldTemplate: z.enum(["grid", "platform", "blank"]),
    name: z.string().min(1, "セッション名を入力してください"),
    description: z.string().optional(),
    maxUsers: z.number().int().min(1, "最低1人以上の設定が必要です"),
    accessLevel: z.number().int().min(1).max(6),
    customSessionId: z.string().optional(),
    hideFromPublicListing: z.boolean(),
    awayKickMinutes: z.number(),
    idleRestartIntervalSeconds: z.number().int(),
    saveOnExit: z.boolean(),
    autoSaveIntervalSeconds: z.number().int(),
    autoSleep: z.boolean(),
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

export default function NewSessionForm() {
  const navigate = useNavigate();
  const notifications = useNotifications();
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
      worldSource: "url",
      worldTemplate: "grid",
      maxUsers: 15,
      accessLevel: 1,
      hideFromPublicListing: false,
      awayKickMinutes: -1,
      idleRestartIntervalSeconds: -1,
      saveOnExit: false,
      autoSaveIntervalSeconds: -1,
      autoSleep: false,
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
      console.error(e);
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
          maxUsers: data.maxUsers,
          accessLevel: data.accessLevel,
          customSessionId: data.customSessionId || "",
          hideFromPublicListing: data.hideFromPublicListing,
          awayKickMinutes: data.awayKickMinutes,
          idleRestartIntervalSeconds: data.idleRestartIntervalSeconds,
          saveOnExit: data.saveOnExit,
          autoSaveIntervalSeconds: data.autoSaveIntervalSeconds,
          autoSleep: data.autoSleep,
        },
      });
      notifications.show("セッションを開始しました", {
        severity: "success",
        autoHideDuration: 3000,
      });
      navigate("/sessions");
    } catch (e) {
      notifications.show(`エラー: ${e instanceof Error ? e.message : e}`, {
        severity: "error",
        autoHideDuration: 3000,
      });
    }
  };

  return (
    <Stack
      component="form"
      noValidate
      autoComplete="off"
      spacing={2}
      onSubmit={handleSubmit(onSubmit)}
    >
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
          <FormControl>
            <FormLabel id="session-form-use-world-url">
              ワールド指定方法
            </FormLabel>
            <RadioGroup
              aria-labelledby="session-form-use-world-url"
              row
              {...field}
            >
              <FormControlLabel
                value="url"
                control={<Radio />}
                label="レコードURLを指定"
              />
              <FormControlLabel
                value="template"
                control={<Radio />}
                label="テンプレートを指定"
              />
            </RadioGroup>
          </FormControl>
        )}
      />
      {worldSource === "url" ? (
        <Grid2 container spacing={2}>
          <Grid2 size={10}>
            <Controller
              name="worldUrl"
              control={control}
              render={({ field }) => (
                <TextField
                  label="レコードURL"
                  fullWidth
                  {...field}
                  error={!!errors.worldUrl}
                  helperText={errors.worldUrl?.message}
                />
              )}
            />
          </Grid2>
          <Grid2 size={2} sx={{ alignItems: "center" }} container>
            <Button
              variant="outlined"
              size="large"
              onClick={handleFetchInfo}
              disabled={isPendingFetchInfo || !hostId || !worldUrl}
            >
              情報取得
            </Button>
          </Grid2>
        </Grid2>
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
          <TextField
            label="セッション名"
            fullWidth
            {...field}
            error={!!errors.name}
            helperText={errors.name?.message}
          />
        )}
      />
      <Controller
        name="description"
        control={control}
        render={({ field }) => (
          <TextField
            label="説明"
            multiline
            fullWidth
            {...field}
            error={!!errors.description}
            helperText={errors.description?.message}
          />
        )}
      />
      <Stack direction="row" spacing={2}>
        <Controller
          name="maxUsers"
          control={control}
          render={({ field }) => (
            <TextField
              label="最大ユーザー数"
              type="number"
              {...field}
              onChange={(e) => {
                const value =
                  e.target.value === "" ? "" : parseInt(e.target.value);
                field.onChange(value);
              }}
              error={!!errors.maxUsers}
              helperText={errors.maxUsers?.message}
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
              error={!!errors.accessLevel}
              helperText={errors.accessLevel?.message}
            />
          )}
        />

        <Controller
          name="hideFromPublicListing"
          control={control}
          render={({ field }) => (
            <FormControlLabel
              label="セッションリストから隠す"
              control={
                <Checkbox
                  checked={field.value}
                  onChange={(e) => field.onChange(e.target.checked)}
                />
              }
            />
          )}
        />
      </Stack>
      <Controller
        name="customSessionId"
        control={control}
        render={({ field }) => (
          <TextField
            label="カスタムセッションID"
            fullWidth
            {...field}
            error={!!errors.customSessionId}
            helperText={errors.customSessionId?.message}
          />
        )}
      />{" "}
      <Controller
        name="awayKickMinutes"
        control={control}
        render={({ field }) => (
          <TextField
            label="AFKキック時間(分)"
            type="number"
            {...field}
            onChange={(e) => {
              const value =
                e.target.value === "" ? "" : parseFloat(e.target.value);
              field.onChange(value);
            }}
            error={!!errors.awayKickMinutes}
            helperText={errors.awayKickMinutes?.message || "-1で無効"}
          />
        )}
      />
      <Stack direction="row" spacing={2}>
        <Controller
          name="autoSaveIntervalSeconds"
          control={control}
          render={({ field }) => (
            <TextField
              label="自動保存間隔(秒)"
              type="number"
              {...field}
              onChange={(e) => {
                const value =
                  e.target.value === "" ? "" : parseInt(e.target.value);
                field.onChange(value);
              }}
              error={!!errors.autoSaveIntervalSeconds}
              helperText={errors.autoSaveIntervalSeconds?.message || "-1で無効"}
            />
          )}
        />

        <Controller
          name="saveOnExit"
          control={control}
          render={({ field }) => (
            <FormControlLabel
              label="セッション終了時に保存"
              control={
                <Checkbox
                  checked={field.value}
                  onChange={(e) => field.onChange(e.target.checked)}
                />
              }
            />
          )}
        />
      </Stack>
      <Stack direction="row" spacing={2}>
        <Controller
          name="idleRestartIntervalSeconds"
          control={control}
          render={({ field }) => (
            <TextField
              label="アイドル時の自動再起動間隔(秒)"
              type="number"
              {...field}
              onChange={(e) => {
                const value =
                  e.target.value === "" ? "" : parseInt(e.target.value);
                field.onChange(value);
              }}
              error={!!errors.idleRestartIntervalSeconds}
              helperText={
                errors.idleRestartIntervalSeconds?.message || "-1で無効"
              }
            />
          )}
        />

        <Controller
          name="autoSleep"
          control={control}
          render={({ field }) => (
            <FormControlLabel
              label="自動スリープ"
              control={
                <Checkbox
                  checked={field.value}
                  onChange={(e) => field.onChange(e.target.checked)}
                />
              }
            />
          )}
        />
      </Stack>
      <Button
        variant="contained"
        type="submit"
        disabled={Object.keys(errors).length > 0}
      >
        セッション開始
      </Button>
    </Stack>
  );
}
