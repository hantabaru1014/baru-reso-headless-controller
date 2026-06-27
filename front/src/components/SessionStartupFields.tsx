import {
  Control,
  Controller,
  FieldErrors,
  useFieldArray,
  UseFormSetValue,
  UseFormWatch,
} from "react-hook-form";
import { SessionFormValues } from "../libs/sessionFormUtils";
import { useMutation } from "@connectrpc/connect-query";
import { fetchWorldInfo } from "../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useEffect, useRef, useState } from "react";
import { useAtomValue } from "jotai";
import { sessionAtom } from "../atoms/sessionAtom";
import { toast } from "sonner";
import { Button, Checkbox, Label } from "./ui";
import { Download, Search, Trash2 } from "lucide-react";
import {
  CheckboxField,
  RadioGroupField,
  SelectField,
  TextField,
  TextareaField,
  UserInfo,
  UserSearchField,
} from "./base";
import { AccessLevels, UserRoles } from "../constants";
import { WorldSearchDialog } from "./WorldSearchDialog";
import { ResoniteUserIcon } from "./ResoniteUserIcon";

type Props = {
  control: Control<SessionFormValues>;
  errors: FieldErrors<SessionFormValues>;
  watch: UseFormWatch<SessionFormValues>;
  setValue: UseFormSetValue<SessionFormValues>;
};

/**
 * NewSessionForm と予約フォーム (START_SESSION) の共通フィールド群。
 * hostId は呼び出し側で別途扱い (新規作成では Dialog、予約フォームでは SelectField)。
 */
export default function SessionStartupFields({
  control,
  errors,
  watch,
  setValue,
}: Props) {
  const hostId = watch("hostId");
  const worldSource = watch("worldSource");
  const worldUrl = watch("worldUrl");

  const { mutateAsync: mutateFetchInfo, isPending: isPendingFetchInfo } =
    useMutation(fetchWorldInfo);
  const [worldSearchDialogOpen, setWorldSearchDialogOpen] = useState(false);

  const {
    fields: defaultUserRoleFields,
    append: appendDefaultUserRole,
    remove: removeDefaultUserRole,
  } = useFieldArray({ control, name: "defaultUserRoles" });

  const {
    fields: autoInviteFields,
    append: appendAutoInvite,
    remove: removeAutoInvite,
  } = useFieldArray({ control, name: "autoInviteUsernames" });

  // ログインユーザーを自動追加
  const session = useAtomValue(sessionAtom);
  const hasAddedCurrentUser = useRef(false);

  useEffect(() => {
    if (hasAddedCurrentUser.current) return;
    if (!session?.user?.resoniteId || !session?.user?.resoniteName) return;

    const { resoniteId, resoniteName, iconUrl } = session.user;

    const existsInAutoInvite = autoInviteFields.some(
      (f) => f.userId === resoniteId,
    );
    if (!existsInAutoInvite) {
      appendAutoInvite({
        userName: resoniteName,
        userId: resoniteId,
        iconUrl,
        joinAllowedOnly: true,
      });
    }

    const existsInDefaultRoles = defaultUserRoleFields.some(
      (f) => f.userName === resoniteName,
    );
    if (!existsInDefaultRoles) {
      appendDefaultUserRole({
        userName: resoniteName,
        role: "Admin",
        iconUrl,
      });
    }

    hasAddedCurrentUser.current = true;
  }, [
    session,
    autoInviteFields,
    defaultUserRoleFields,
    appendAutoInvite,
    appendDefaultUserRole,
  ]);

  const handleDefaultUserRoleSelect = (user: UserInfo) => {
    const exists = defaultUserRoleFields.some((f) => f.userName === user.name);
    if (!exists) {
      appendDefaultUserRole({
        userName: user.name,
        role: "Guest",
        iconUrl: user.iconUrl,
      });
    }
  };

  const handleAutoInviteSelect = (user: UserInfo) => {
    const exists = autoInviteFields.some((f) => f.userId === user.id);
    if (!exists) {
      appendAutoInvite({
        userName: user.name,
        userId: user.id,
        iconUrl: user.iconUrl,
        joinAllowedOnly: false,
      });
    }
  };

  const handleFetchInfo = async (url?: string) => {
    const targetUrl = url ?? worldUrl;
    if (!hostId || !targetUrl) return;

    try {
      const data = await mutateFetchInfo({ hostId, url: targetUrl });
      setValue("name", data.name);
      setValue("description", data.description || "");
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : "ワールド情報の取得に失敗しました",
      );
    }
  };

  const handleWorldUrlBlur = () => {
    if (hostId && worldUrl) {
      handleFetchInfo();
    }
  };

  return (
    <>
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
        <>
          <div className="flex gap-2 items-center">
            <div className="flex-grow">
              <Controller
                name="worldUrl"
                control={control}
                render={({ field }) => (
                  <TextField
                    label="レコードURL"
                    error={errors.worldUrl?.message}
                    {...field}
                    onBlur={() => {
                      field.onBlur();
                      handleWorldUrlBlur();
                    }}
                  />
                )}
              />
            </div>
            <Button
              type="button"
              variant="outline"
              size="lg"
              className="shrink-0"
              onClick={() => handleFetchInfo()}
              disabled={isPendingFetchInfo || !hostId || !worldUrl}
            >
              <Download className="h-4 w-4" />
              <span className="hidden sm:inline">情報取得</span>
            </Button>
            <Button
              type="button"
              variant="outline"
              size="lg"
              className="shrink-0"
              onClick={() => setWorldSearchDialogOpen(true)}
            >
              <Search className="h-4 w-4" />
              <span className="hidden sm:inline">ワールド検索</span>
            </Button>
          </div>
          <WorldSearchDialog
            open={worldSearchDialogOpen}
            onClose={() => setWorldSearchDialogOpen(false)}
            onSelect={(selectedWorldUrl) => {
              setValue("worldUrl", selectedWorldUrl);
              setWorldSearchDialogOpen(false);
              handleFetchInfo(selectedWorldUrl);
            }}
            hostId={hostId}
          />
        </>
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
            richTextMode="full"
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
            richTextMode="full"
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
        <div className="space-y-1">
          <UserSearchField
            hostId={hostId}
            onUserSelect={handleAutoInviteSelect}
            placeholder="ユーザーを検索して追加"
            label="自動招待ユーザ"
          />
          {autoInviteFields.length > 0 && (
            <div className="space-y-2 rounded-md border p-2">
              {autoInviteFields.map((field, index) => (
                <div
                  key={field.id}
                  className="flex items-center gap-2 p-2 rounded-md border bg-muted/50"
                >
                  <ResoniteUserIcon
                    iconUrl={field.iconUrl}
                    alt={`${field.userName}のアイコン`}
                    className="h-8 w-8"
                  />
                  <span className="flex-1 text-sm">{field.userName}</span>
                  <Controller
                    name={`autoInviteUsernames.${index}.joinAllowedOnly`}
                    control={control}
                    render={({ field: checkboxField }) => (
                      <div className="flex items-center gap-1.5">
                        <Checkbox
                          id={`joinAllowedOnly-${index}`}
                          checked={checkboxField.value}
                          onCheckedChange={checkboxField.onChange}
                        />
                        <Label
                          htmlFor={`joinAllowedOnly-${index}`}
                          className="text-sm"
                        >
                          参加許可のみ
                        </Label>
                      </div>
                    )}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => removeAutoInvite(index)}
                    title="削除"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>

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

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="space-y-1">
          <UserSearchField
            hostId={hostId}
            onUserSelect={handleDefaultUserRoleSelect}
            placeholder="ユーザーを検索して追加"
            label="デフォルトユーザーロール"
          />
          {defaultUserRoleFields.length > 0 && (
            <div className="space-y-2 rounded-md border p-2">
              {defaultUserRoleFields.map((field, index) => (
                <div
                  key={field.id}
                  className="flex items-center gap-2 p-2 rounded-md border bg-muted/50"
                >
                  <ResoniteUserIcon
                    iconUrl={field.iconUrl}
                    alt={`${field.userName}のアイコン`}
                    className="h-8 w-8"
                  />
                  <span className="flex-1 text-sm">{field.userName}</span>
                  <Controller
                    name={`defaultUserRoles.${index}.role`}
                    control={control}
                    render={({ field: roleField }) => (
                      <SelectField
                        options={UserRoles.map((r) => r)}
                        selectedId={roleField.value ?? ""}
                        onChange={(option) => roleField.onChange(option.id)}
                        minWidth="7rem"
                      />
                    )}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => removeDefaultUserRole(index)}
                    title="削除"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
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
    </>
  );
}
