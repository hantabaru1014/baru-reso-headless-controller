import { Input } from "../ui";
import { callUnaryMethod, useTransport } from "@connectrpc/connect-query";
import { useInfiniteQuery } from "@tanstack/react-query";
import { listContacts } from "../../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Loader2 } from "lucide-react";
import { cn } from "@/libs/cssUtils";
import { ResoniteUserIcon } from "../ResoniteUserIcon";
import type { ContactListPanelProps } from "./types";

export function ContactListPanel({
  accountId,
  enabled,
  selectedContact,
  onSelectContact,
}: ContactListPanelProps) {
  const transport = useTransport();
  const [contactSearch, setContactSearch] = useState("");
  const contactsScrollRef = useRef<HTMLDivElement>(null);

  // Contact list query with infinite scroll
  const {
    data: contactsData,
    fetchNextPage: fetchNextContacts,
    hasNextPage: hasMoreContacts,
    isFetchingNextPage: isFetchingMoreContacts,
    isPending: isLoadingContacts,
    isError: isContactsError,
    error: contactsError,
  } = useInfiniteQuery({
    queryKey: ["contacts", accountId],
    queryFn: async ({ pageParam }) => {
      const response = await callUnaryMethod(transport, listContacts, {
        headlessAccountId: accountId,
        limit: 50,
        cursor: pageParam?.cursor,
      });
      return {
        contacts: response.contacts,
        nextCursor: response.nextCursor,
      };
    },
    initialPageParam: undefined as { cursor?: string } | undefined,
    getNextPageParam: (lastPage) => {
      if (!lastPage.nextCursor) return undefined;
      return { cursor: lastPage.nextCursor };
    },
    enabled,
    refetchOnMount: "always",
  });

  const contacts = useMemo(
    () => contactsData?.pages.flatMap((page) => page.contacts) ?? [],
    [contactsData?.pages],
  );

  // Filter contacts by search term (name or id)
  const filteredContacts = useMemo(() => {
    if (!contactSearch.trim()) return contacts;
    const searchLower = contactSearch.toLowerCase();
    return contacts.filter(
      (contact) =>
        contact.name.toLowerCase().includes(searchLower) ||
        contact.id.toLowerCase().includes(searchLower),
    );
  }, [contacts, contactSearch]);

  // Contacts virtual scrolling
  const contactsVirtualizer = useVirtualizer({
    count: filteredContacts.length,
    getScrollElement: () => contactsScrollRef.current,
    estimateSize: () => 48,
    overscan: 5,
    getItemKey: (index) => filteredContacts[index]?.id ?? index,
  });

  // Force virtualizer to remeasure when enabled or contacts data changes
  useEffect(() => {
    if (enabled && filteredContacts.length > 0) {
      requestAnimationFrame(() => {
        contactsVirtualizer.measure();
      });
    }
  }, [enabled, filteredContacts.length, contactsVirtualizer]);

  // Handle contacts scroll for loading more
  useEffect(() => {
    const container = contactsScrollRef.current;
    if (!container) return;

    const handleScroll = () => {
      if (
        container.scrollHeight - container.scrollTop - container.clientHeight <
          100 &&
        hasMoreContacts &&
        !isFetchingMoreContacts
      ) {
        fetchNextContacts();
      }
    };

    container.addEventListener("scroll", handleScroll);
    return () => container.removeEventListener("scroll", handleScroll);
  }, [fetchNextContacts, hasMoreContacts, isFetchingMoreContacts]);

  // Reset search when disabled
  useEffect(() => {
    if (!enabled) {
      setContactSearch("");
    }
  }, [enabled]);

  return (
    <div className="h-full flex flex-col border rounded-l-md">
      {/* Search field */}
      <div className="p-2 border-b">
        <Input
          value={contactSearch}
          onChange={(e) => setContactSearch(e.target.value)}
          placeholder="名前またはIDで検索..."
          className="h-8 text-sm"
        />
      </div>
      {/* Contact list */}
      <div ref={contactsScrollRef} className="flex-1 overflow-auto">
        {isContactsError ? (
          <div className="p-4 text-center text-destructive text-sm">
            <p className="font-medium mb-1">エラーが発生しました</p>
            <p className="text-xs text-muted-foreground">
              {contactsError instanceof Error
                ? contactsError.message
                : "このアカウントでホストが起動していない可能性があります"}
            </p>
          </div>
        ) : isLoadingContacts ? (
          <div className="flex justify-center p-4">
            <Loader2 className="animate-spin" />
          </div>
        ) : contacts.length === 0 ? (
          <div className="p-4 text-center text-muted-foreground text-sm">
            コンタクトがありません
          </div>
        ) : filteredContacts.length === 0 ? (
          <div className="p-4 text-center text-muted-foreground text-sm">
            検索結果がありません
          </div>
        ) : (
          <div
            style={{
              height: contactsVirtualizer.getTotalSize(),
              position: "relative",
            }}
          >
            {contactsVirtualizer.getVirtualItems().map((virtualItem) => {
              const contact = filteredContacts[virtualItem.index];
              return (
                <button
                  key={contact.id}
                  onClick={() => onSelectContact(contact)}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    height: virtualItem.size,
                    transform: `translateY(${virtualItem.start}px)`,
                  }}
                  className={cn(
                    "flex items-center gap-2 px-2 hover:bg-muted/50 transition-colors",
                    selectedContact?.id === contact.id && "bg-muted",
                  )}
                >
                  <ResoniteUserIcon
                    iconUrl={contact.iconUrl}
                    alt={contact.name}
                    className="h-8 w-8"
                  />
                  <span className="truncate text-sm">{contact.name}</span>
                </button>
              );
            })}
          </div>
        )}
        {isFetchingMoreContacts && (
          <div className="flex justify-center p-2">
            <Loader2 className="h-4 w-4 animate-spin" />
          </div>
        )}
      </div>
    </div>
  );
}
