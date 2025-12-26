import type { SessionInviteData } from "./types";

export const parseSessionInvite = (
  content: string,
): SessionInviteData | null => {
  try {
    const data = JSON.parse(content);
    return {
      name: data.name ?? "",
      sessionId: data.sessionId ?? "",
      hostUsername: data.hostUsername ?? "",
      joinedUsers: data.joinedUsers ?? 0,
      activeUsers: data.activeUsers ?? 0,
      thumbnailUrl: data.thumbnailUrl ?? null,
      lastUpdate: data.lastUpdate ?? "",
    };
  } catch {
    return null;
  }
};

export const formatSessionTime = (isoString: string) => {
  if (!isoString) return "";
  const date = new Date(isoString);
  return date.toLocaleString("ja-JP", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });
};

export const formatMessageTime = (
  sendTime: { seconds: bigint | number } | undefined,
) => {
  if (!sendTime) return "";
  const date = new Date(Number(sendTime.seconds) * 1000);
  return date.toLocaleString("ja-JP", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
};
