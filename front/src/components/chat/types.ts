import type { UserInfo } from "../../../pbgen/hdlctrl/v1/controller_pb";

export interface SessionInviteData {
  name: string;
  sessionId: string;
  hostUsername: string;
  joinedUsers: number;
  activeUsers: number;
  thumbnailUrl: string | null;
  lastUpdate: string;
}

export type MessagePageParam = {
  direction: "before" | "after" | "init";
  cursorId?: string;
};

export interface ContactListPanelProps {
  // アカウントは (groupId, accountId) の組で特定する
  groupId: string;
  accountId: string;
  enabled: boolean;
  selectedContact: UserInfo | null;
  onSelectContact: (contact: UserInfo) => void;
}

export interface ChatMessagesPanelProps {
  groupId: string;
  accountId: string;
  contact: UserInfo | null;
  enabled: boolean;
  onNavigateToSession?: (sessionId: string) => void;
  showPlaceholder?: boolean;
  className?: string;
}

export interface ChatDialogProps {
  open: boolean;
  onClose?: () => void;
  groupId: string;
  accountId: string;
  accountName: string;
}

export interface DirectChatDialogProps {
  open: boolean;
  onClose?: () => void;
  groupId: string;
  accountId: string;
  accountName: string;
  contact: UserInfo;
}
