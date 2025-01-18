export const AccessLevels = [
  { id: "1", label: "プライベート(招待のみ)", value: 1 },
  { id: "2", label: "LAN内", value: 2 },
  { id: "3", label: "フレンド", value: 3 },
  { id: "4", label: "フレンド＋", value: 4 },
  { id: "5", label: "ログインユーザー", value: 5 },
  { id: "6", label: "誰でも", value: 6 },
] as const;

export const UserRoles = [
  { id: "Admin", label: "Admin" },
  { id: "Builder", label: "Builder" },
  { id: "Moderator", label: "Moderator" },
  { id: "Guest", label: "Guest" },
  { id: "Spectator", label: "Spectator" },
] as const;
