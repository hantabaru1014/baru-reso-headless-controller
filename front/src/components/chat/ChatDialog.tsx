import { Dialog, DialogContent, DialogHeader, DialogTitle } from "../ui";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../ui/resizable";
import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router";
import type { UserInfo } from "../../../pbgen/hdlctrl/v1/controller_pb";
import type { ChatDialogProps } from "./types";
import { ContactListPanel } from "./ContactListPanel";
import { ChatMessagesPanel } from "./ChatMessagesPanel";

export function ChatDialog({
  open,
  onClose,
  accountId,
  accountName,
}: ChatDialogProps) {
  const navigate = useNavigate();
  const [selectedContact, setSelectedContact] = useState<UserInfo | null>(null);

  // Reset state when dialog closes
  useEffect(() => {
    if (!open) {
      setSelectedContact(null);
    }
  }, [open]);

  const handleNavigateToSession = useCallback(
    (sessionId: string) => {
      navigate(`/sessions/${sessionId}`);
      onClose?.();
    },
    [navigate, onClose],
  );

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose?.()}>
      <DialogContent className="sm:max-w-[1000px] h-[80vh] max-h-[800px] flex flex-col">
        <DialogHeader>
          <DialogTitle>チャット - {accountName}</DialogTitle>
        </DialogHeader>

        <ResizablePanelGroup direction="horizontal" className="flex-1 min-h-0">
          {/* Left panel: Contact list */}
          <ResizablePanel defaultSize={30} minSize={20}>
            <ContactListPanel
              accountId={accountId}
              enabled={open}
              selectedContact={selectedContact}
              onSelectContact={setSelectedContact}
            />
          </ResizablePanel>

          <ResizableHandle withHandle />

          {/* Right panel: Messages */}
          <ResizablePanel defaultSize={70} minSize={40}>
            <ChatMessagesPanel
              accountId={accountId}
              contact={selectedContact}
              enabled={open}
              onNavigateToSession={handleNavigateToSession}
              className="border rounded-r-md"
            />
          </ResizablePanel>
        </ResizablePanelGroup>
      </DialogContent>
    </Dialog>
  );
}
