import { create } from "@bufbuild/protobuf";
import {
  NetworkProtocol,
  WorldStartupParameters,
  WorldStartupParametersSchema,
} from "../../pbgen/headless/v1/headless_pb";
import { z } from "zod";
import type { TFunction } from "i18next";

/**
 * プロトコルごとのポート指定 (force_ports) のフィールド定義。
 * フォームのフィールド名・クエリパラメータ名・proto の NetworkProtocol の
 * 対応をここ 1 箇所で定義し、変換と描画はこれを回して行う。
 */
export const FORCE_PORT_FIELDS = [
  {
    name: "forcePortLnl",
    protocol: NetworkProtocol.LNL,
    label: "forcePorts (LNL)",
    helperTextKey: undefined,
  },
  {
    name: "forcePortQuic",
    protocol: NetworkProtocol.QUIC,
    label: "forcePorts (QUIC)",
    helperTextKey: "sessionStartupFields.forcePortQuicHelp",
  },
  {
    name: "forcePortTcp",
    protocol: NetworkProtocol.TCP,
    label: "forcePorts (TCP)",
    helperTextKey: undefined,
  },
] as const;

type ForcePortFieldName = (typeof FORCE_PORT_FIELDS)[number]["name"];

/** ポート番号の入力。範囲外はヘッドレス側で無視されるのでここで弾く。 */
const portSchema = (t: TFunction) =>
  z
    .number()
    .int()
    .min(1, t("sessionFormUtils.portRange"))
    .max(65535, t("sessionFormUtils.portRange"))
    .optional();

/**
 * 新規セッション/予約フォーム共通の zod スキーマ。
 * バリデーションメッセージを i18n 化するため、`t` を受け取るファクトリ関数にしている。
 */
export const makeSessionFormSchema = (t: TFunction) =>
  z
    .object({
      hostId: z.string().min(1, t("sessionFormUtils.hostRequired")),
      worldSource: z.enum(["url", "template"]),
      name: z.string().min(1, t("sessionFormUtils.nameRequired")),
      customSessionId: z.string().optional(),
      description: z.string().optional(),
      tags: z.string().optional(),
      maxUsers: z.number().int().min(1, t("sessionFormUtils.maxUsersMin")),
      accessLevel: z.number().int().min(1).max(6),
      worldUrl: z.string().optional(),
      worldTemplate: z.enum(["grid", "platform", "blank"]),
      autoInviteUsernames: z
        .array(
          z.object({
            userName: z.string(),
            userId: z.string(),
            iconUrl: z.string().optional(),
            joinAllowedOnly: z.boolean(),
          }),
        )
        .optional(),
      hideFromPublicListing: z.boolean(),
      defaultUserRoles: z
        .array(
          z.object({
            role: z.string(),
            userName: z.string(),
            iconUrl: z.string().optional(),
          }),
        )
        .optional(),
      awayKickMinutes: z.number(),
      idleRestartIntervalSeconds: z.number().int(),
      saveOnExit: z.boolean(),
      autoSaveIntervalSeconds: z.number().int(),
      autoSleep: z.boolean(),
      inviteRequestHandlerUsernames: z.string().optional(),
      forcePortLnl: portSchema(t),
      forcePortQuic: portSchema(t),
      forcePortTcp: portSchema(t),
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
            if (!value) return true;
            return /^[^/]+\/[^/]+$/.test(value);
          },
          { message: t("sessionFormUtils.recordIdFormat") },
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
      { message: t("sessionFormUtils.worldUrlRequired"), path: ["worldUrl"] },
    );

export type SessionFormValues = z.infer<
  ReturnType<typeof makeSessionFormSchema>
>;

const processCSV = (csv: string | undefined): string[] =>
  csv
    ?.split(",")
    .map((n) => n.trim())
    .filter((n) => n) || [];

/**
 * プロトコルごとのポート入力を force_ports に変換する。
 * 未入力 (undefined / 0) のプロトコルはヘッドレス側でランダムなポートが使われる。
 */
const processForcePorts = (data: Partial<Record<ForcePortFieldName, number>>) =>
  FORCE_PORT_FIELDS.filter(({ name }) => !!data[name]).map(
    ({ name, protocol }) => ({ protocol, port: data[name] }),
  );

const processRecordId = (
  recordId: string | undefined,
): { id: string; ownerId: string } | undefined => {
  if (!recordId) return undefined;
  const parts = recordId.split("/");
  if (parts.length !== 2) return undefined;
  return { ownerId: parts[0], id: parts[1] };
};

/**
 * SessionFormValues → WorldStartupParameters proto に変換する。
 * StartWorld と Schedule START の両方で利用される。
 */
export function buildStartWorldParameters(
  data: SessionFormValues,
): WorldStartupParameters {
  return create(WorldStartupParametersSchema, {
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
    autoInviteUsernames:
      data.autoInviteUsernames
        ?.filter((u) => !u.joinAllowedOnly)
        .map((u) => u.userName) || [],
    joinAllowedUserIds:
      data.autoInviteUsernames
        ?.filter((u) => u.joinAllowedOnly)
        .map((u) => u.userId) || [],
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
    forcePorts: processForcePorts(data),
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
  });
}

export function removeUndefined<T extends object>(obj: T): Partial<T> {
  return Object.fromEntries(
    Object.entries(obj).filter(([, v]) => v !== undefined),
  ) as Partial<T>;
}

/**
 * フォームの入力値の型定義（クエリパラメータから読み取り可能なフィールド）
 */
export interface SessionFormPrefillValues {
  worldSource?: "url" | "template";
  worldUrl?: string;
  worldTemplate?: "grid" | "platform" | "blank";
  name?: string;
  customSessionId?: string;
  description?: string;
  tags?: string;
  maxUsers?: number;
  accessLevel?: number;
  hideFromPublicListing?: boolean;
  autoInviteUsernames?: Array<{
    userName: string;
    userId: string;
    iconUrl?: string;
    joinAllowedOnly: boolean;
  }>;
  autoInviteMessage?: string;
  defaultUserRoles?: Array<{
    role: string;
    userName: string;
    iconUrl?: string;
  }>;
  keepOriginalRoles?: boolean;
  parentSessionIds?: string;
  overrideCorrespondingWorldId?: string;
  autoRecover?: boolean;
  forcedRestartIntervalSeconds?: number;
  autoSleep?: boolean;
  idleRestartIntervalSeconds?: number;
  awayKickMinutes?: number;
  saveOnExit?: boolean;
  autoSaveIntervalSeconds?: number;
  roleCloudVariable?: string;
  allowUserCloudVariable?: string;
  denyUserCloudVariable?: string;
  requiredUserJoinCloudVariable?: string;
  requiredUserJoinCloudVariableDenyMessage?: string;
  forcePortLnl?: number;
  forcePortQuic?: number;
  forcePortTcp?: number;
  mobileFriendly?: boolean;
  useCustomJoinVerifier?: boolean;
  inviteRequestHandlerUsernames?: string;
}

/**
 * セッションフォームのデフォルト値
 */
export const DEFAULT_SESSION_FORM_VALUES = {
  hostId: "",
  worldSource: "url" as const,
  worldTemplate: "grid" as const,
  worldUrl: "",
  name: "",
  customSessionId: "",
  description: "",
  maxUsers: 15,
  accessLevel: 1,
  hideFromPublicListing: false,
  tags: "",
  autoInviteUsernames: [] as Array<{
    userName: string;
    userId: string;
    iconUrl?: string;
    joinAllowedOnly: boolean;
  }>,
  autoInviteMessage: "",
  defaultUserRoles: [] as Array<{
    role: string;
    userName: string;
    iconUrl?: string;
  }>,
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
  forcePortLnl: undefined as number | undefined,
  forcePortQuic: undefined as number | undefined,
  forcePortTcp: undefined as number | undefined,
  overrideCorrespondingWorldId: "",
  roleCloudVariable: "",
  allowUserCloudVariable: "",
  denyUserCloudVariable: "",
  requiredUserJoinCloudVariable: "",
  requiredUserJoinCloudVariableDenyMessage: "",
};

const defaults = DEFAULT_SESSION_FORM_VALUES;

/**
 * WorldStartupParameters を URLSearchParams に変換する
 * デフォルト値と異なる値のみクエリパラメータに含める
 */
export function startupParamsToSearchParams(
  params: WorldStartupParameters | undefined,
): URLSearchParams {
  const searchParams = new URLSearchParams();
  if (!params) return searchParams;

  // 文字列フィールド（空文字がデフォルト）
  if (params.name) searchParams.set("name", params.name);
  if (params.description) searchParams.set("description", params.description);
  if (params.customSessionId)
    searchParams.set("customSessionId", params.customSessionId);
  if (params.autoInviteMessage)
    searchParams.set("autoInviteMessage", params.autoInviteMessage);
  if (params.roleCloudVariable)
    searchParams.set("roleCloudVariable", params.roleCloudVariable);
  if (params.allowUserCloudVariable)
    searchParams.set("allowUserCloudVariable", params.allowUserCloudVariable);
  if (params.denyUserCloudVariable)
    searchParams.set("denyUserCloudVariable", params.denyUserCloudVariable);
  if (params.requiredUserJoinCloudVariable)
    searchParams.set(
      "requiredUserJoinCloudVariable",
      params.requiredUserJoinCloudVariable,
    );
  if (params.requiredUserJoinCloudVariableDenyMessage)
    searchParams.set(
      "requiredUserJoinCloudVariableDenyMessage",
      params.requiredUserJoinCloudVariableDenyMessage,
    );

  // 数値フィールド（デフォルトと異なる場合のみ）
  if (params.maxUsers !== undefined && params.maxUsers !== defaults.maxUsers)
    searchParams.set("maxUsers", String(params.maxUsers));
  if (params.accessLevel !== defaults.accessLevel)
    searchParams.set("accessLevel", String(params.accessLevel));
  if (params.awayKickMinutes !== defaults.awayKickMinutes)
    searchParams.set("awayKickMinutes", String(params.awayKickMinutes));
  if (params.idleRestartIntervalSeconds !== defaults.idleRestartIntervalSeconds)
    searchParams.set(
      "idleRestartIntervalSeconds",
      String(params.idleRestartIntervalSeconds),
    );
  if (params.autoSaveIntervalSeconds !== defaults.autoSaveIntervalSeconds)
    searchParams.set(
      "autoSaveIntervalSeconds",
      String(params.autoSaveIntervalSeconds),
    );
  if (
    params.forcedRestartIntervalSeconds !==
    defaults.forcedRestartIntervalSeconds
  )
    searchParams.set(
      "forcedRestartIntervalSeconds",
      String(params.forcedRestartIntervalSeconds),
    );
  // force_ports が空の場合のみ、旧フィールドの forcePort を LNL のポートとして扱う
  // (ヘッドレス側の解釈と揃えている)
  const forcePorts = params.forcePorts.length
    ? params.forcePorts
    : [{ protocol: NetworkProtocol.LNL, port: params.forcePort }];
  const forcePortKeys = new Map<NetworkProtocol, string>(
    FORCE_PORT_FIELDS.map(({ protocol, name }) => [protocol, name]),
  );
  // 同じプロトコルが複数ある場合は後勝ち (set が上書きする)
  for (const { protocol, port } of forcePorts) {
    const key = forcePortKeys.get(protocol);
    if (key && port) searchParams.set(key, String(port));
  }

  // 真偽値フィールド（デフォルトと異なる場合のみ）
  if (params.hideFromPublicListing !== defaults.hideFromPublicListing)
    searchParams.set(
      "hideFromPublicListing",
      String(params.hideFromPublicListing),
    );
  if (params.saveOnExit !== defaults.saveOnExit)
    searchParams.set("saveOnExit", String(params.saveOnExit));
  if (params.autoSleep !== defaults.autoSleep)
    searchParams.set("autoSleep", String(params.autoSleep));
  if (params.autoRecover !== defaults.autoRecover)
    searchParams.set("autoRecover", String(params.autoRecover));
  if (params.useCustomJoinVerifier !== defaults.useCustomJoinVerifier)
    searchParams.set(
      "useCustomJoinVerifier",
      String(params.useCustomJoinVerifier),
    );
  if (params.mobileFriendly !== defaults.mobileFriendly)
    searchParams.set("mobileFriendly", String(params.mobileFriendly));
  if (params.keepOriginalRoles !== defaults.keepOriginalRoles)
    searchParams.set("keepOriginalRoles", String(params.keepOriginalRoles));

  // worldSource判定
  if (params.loadWorld?.case === "loadWorldUrl") {
    searchParams.set("worldSource", "url");
    searchParams.set("worldUrl", params.loadWorld.value);
  } else if (params.loadWorld?.case === "loadWorldPresetName") {
    searchParams.set("worldSource", "template");
    searchParams.set("worldTemplate", params.loadWorld.value);
  }

  // 配列→カンマ区切り（空配列がデフォルト）
  if (params.tags.length > 0) {
    searchParams.set("tags", params.tags.join(", "));
  }
  if (params.parentSessionIds.length > 0) {
    searchParams.set("parentSessionIds", params.parentSessionIds.join(", "));
  }
  if (params.inviteRequestHandlerUsernames.length > 0) {
    searchParams.set(
      "inviteRequestHandlerUsernames",
      params.inviteRequestHandlerUsernames.join(", "),
    );
  }

  // overrideCorrespondingWorldId (RecordId → "ownerId/id")
  if (params.overrideCorrespondingWorldId) {
    searchParams.set(
      "overrideCorrespondingWorldId",
      `${params.overrideCorrespondingWorldId.ownerId}/${params.overrideCorrespondingWorldId.id}`,
    );
  }

  // オブジェクト配列→JSON（空配列がデフォルト）
  const autoInviteData = [
    ...params.autoInviteUsernames.map((name) => ({
      userName: name,
      userId: "",
      joinAllowedOnly: false,
    })),
    ...params.joinAllowedUserIds.map((id) => ({
      userName: "",
      userId: id,
      joinAllowedOnly: true,
    })),
  ];
  if (autoInviteData.length > 0) {
    searchParams.set("autoInviteUsernames", JSON.stringify(autoInviteData));
  }

  if (params.defaultUserRoles.length > 0) {
    searchParams.set(
      "defaultUserRoles",
      JSON.stringify(
        params.defaultUserRoles.map((r) => ({
          userName: r.userName,
          role: r.role,
        })),
      ),
    );
  }

  return searchParams;
}

/**
 * URLSearchParams をフォームの初期値に変換する
 */
export function searchParamsToFormValues(
  params: URLSearchParams,
): SessionFormPrefillValues {
  const result: SessionFormPrefillValues = {};

  // 文字列フィールド
  const name = params.get("name");
  if (name) result.name = name;

  const description = params.get("description");
  if (description) result.description = description;

  const customSessionId = params.get("customSessionId");
  if (customSessionId) result.customSessionId = customSessionId;

  const worldUrl = params.get("worldUrl");
  if (worldUrl) result.worldUrl = worldUrl;

  const worldTemplate = params.get("worldTemplate");
  if (
    worldTemplate === "grid" ||
    worldTemplate === "platform" ||
    worldTemplate === "blank"
  ) {
    result.worldTemplate = worldTemplate;
  }

  const worldSource = params.get("worldSource");
  if (worldSource === "url" || worldSource === "template") {
    result.worldSource = worldSource;
  }

  const tags = params.get("tags");
  if (tags) result.tags = tags;

  const parentSessionIds = params.get("parentSessionIds");
  if (parentSessionIds) result.parentSessionIds = parentSessionIds;

  const inviteRequestHandlerUsernames = params.get(
    "inviteRequestHandlerUsernames",
  );
  if (inviteRequestHandlerUsernames)
    result.inviteRequestHandlerUsernames = inviteRequestHandlerUsernames;

  const overrideCorrespondingWorldId = params.get(
    "overrideCorrespondingWorldId",
  );
  if (overrideCorrespondingWorldId)
    result.overrideCorrespondingWorldId = overrideCorrespondingWorldId;

  const autoInviteMessage = params.get("autoInviteMessage");
  if (autoInviteMessage) result.autoInviteMessage = autoInviteMessage;

  const roleCloudVariable = params.get("roleCloudVariable");
  if (roleCloudVariable) result.roleCloudVariable = roleCloudVariable;

  const allowUserCloudVariable = params.get("allowUserCloudVariable");
  if (allowUserCloudVariable)
    result.allowUserCloudVariable = allowUserCloudVariable;

  const denyUserCloudVariable = params.get("denyUserCloudVariable");
  if (denyUserCloudVariable)
    result.denyUserCloudVariable = denyUserCloudVariable;

  const requiredUserJoinCloudVariable = params.get(
    "requiredUserJoinCloudVariable",
  );
  if (requiredUserJoinCloudVariable)
    result.requiredUserJoinCloudVariable = requiredUserJoinCloudVariable;

  const requiredUserJoinCloudVariableDenyMessage = params.get(
    "requiredUserJoinCloudVariableDenyMessage",
  );
  if (requiredUserJoinCloudVariableDenyMessage)
    result.requiredUserJoinCloudVariableDenyMessage =
      requiredUserJoinCloudVariableDenyMessage;

  // 数値フィールド
  const maxUsers = params.get("maxUsers");
  if (maxUsers) result.maxUsers = parseInt(maxUsers, 10);

  const accessLevel = params.get("accessLevel");
  if (accessLevel) result.accessLevel = parseInt(accessLevel, 10);

  const awayKickMinutes = params.get("awayKickMinutes");
  if (awayKickMinutes) result.awayKickMinutes = parseFloat(awayKickMinutes);

  const idleRestartIntervalSeconds = params.get("idleRestartIntervalSeconds");
  if (idleRestartIntervalSeconds)
    result.idleRestartIntervalSeconds = parseInt(
      idleRestartIntervalSeconds,
      10,
    );

  const autoSaveIntervalSeconds = params.get("autoSaveIntervalSeconds");
  if (autoSaveIntervalSeconds)
    result.autoSaveIntervalSeconds = parseInt(autoSaveIntervalSeconds, 10);

  const forcedRestartIntervalSeconds = params.get(
    "forcedRestartIntervalSeconds",
  );
  if (forcedRestartIntervalSeconds)
    result.forcedRestartIntervalSeconds = parseInt(
      forcedRestartIntervalSeconds,
      10,
    );

  for (const { name } of FORCE_PORT_FIELDS) {
    const forcePort = params.get(name);
    if (forcePort) result[name] = parseInt(forcePort, 10);
  }

  // forcePort は旧形式のリンク互換 (LNL のポートとして扱う)
  const legacyForcePort = params.get("forcePort");
  if (legacyForcePort && result.forcePortLnl === undefined)
    result.forcePortLnl = parseInt(legacyForcePort, 10);

  // 真偽値フィールド
  const hideFromPublicListing = params.get("hideFromPublicListing");
  if (hideFromPublicListing)
    result.hideFromPublicListing = hideFromPublicListing === "true";

  const saveOnExit = params.get("saveOnExit");
  if (saveOnExit) result.saveOnExit = saveOnExit === "true";

  const autoSleep = params.get("autoSleep");
  if (autoSleep) result.autoSleep = autoSleep === "true";

  const autoRecover = params.get("autoRecover");
  if (autoRecover) result.autoRecover = autoRecover === "true";

  const useCustomJoinVerifier = params.get("useCustomJoinVerifier");
  if (useCustomJoinVerifier)
    result.useCustomJoinVerifier = useCustomJoinVerifier === "true";

  const mobileFriendly = params.get("mobileFriendly");
  if (mobileFriendly) result.mobileFriendly = mobileFriendly === "true";

  const keepOriginalRoles = params.get("keepOriginalRoles");
  if (keepOriginalRoles)
    result.keepOriginalRoles = keepOriginalRoles === "true";

  // JSON配列フィールド
  const autoInviteUsernames = params.get("autoInviteUsernames");
  if (autoInviteUsernames) {
    try {
      result.autoInviteUsernames = JSON.parse(autoInviteUsernames);
    } catch {
      // パース失敗時は無視
    }
  }

  const defaultUserRoles = params.get("defaultUserRoles");
  if (defaultUserRoles) {
    try {
      result.defaultUserRoles = JSON.parse(defaultUserRoles);
    } catch {
      // パース失敗時は無視
    }
  }

  return result;
}
