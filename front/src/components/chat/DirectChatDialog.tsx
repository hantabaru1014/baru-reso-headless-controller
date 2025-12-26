import { Dialog, DialogContent, DialogHeader, DialogTitle } from "../ui";
import { useCallback } from "react";
import { useNavigate } from "react-router";
import { ResoniteUserIcon } from "../ResoniteUserIcon";
import type { DirectChatDialogProps } from "./types";
import { ChatMessagesPanel } from "./ChatMessagesPanel";

export function DirectChatDialog({
  open,
  onClose,
  accountId,
  accountName,
  contact,
}: DirectChatDialogProps) {
  const navigate = useNavigate();

  const handleNavigateToSession = useCallback(
    (sessionId: string) => {
      navigate(`/sessions/${sessionId}`);
      onClose?.();
    },
    [navigate, onClose],
  );

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose?.()}>
      <DialogContent className="sm:max-w-[600px] h-[70vh] max-h-[700px] flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ResoniteUserIcon
              iconUrl={contact.iconUrl}
              alt={contact.name}
              className="h-6 w-6"
            />
            <span>
              {contact.name} - {accountName}
            </span>
          </DialogTitle>
        </DialogHeader>

        <ChatMessagesPanel
          accountId={accountId}
          contact={contact}
          enabled={open}
          onNavigateToSession={handleNavigateToSession}
          showPlaceholder={false}
          className="flex-1 min-h-0 border rounded-md"
        />
      </DialogContent>
    </Dialog>
  );
}
