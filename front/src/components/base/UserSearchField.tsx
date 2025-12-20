import { useId, useState } from "react";
import { useMutation } from "@connectrpc/connect-query";
import { searchUserInfo } from "../../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { Input } from "../ui/input";
import { Search } from "lucide-react";
import { UserList, UserInfo } from "./UserList";
import { ScrollBase } from "./ScrollBase";
import { Popover, PopoverContent, PopoverTrigger } from "../ui/popover";

export type { UserInfo };

interface UserSearchFieldProps {
  hostId?: string;
  onUserSelect: (user: UserInfo) => void;
  placeholder?: string;
  disabled?: boolean;
  label?: string;
  noHostMessage?: string;
}

export function UserSearchField({
  hostId,
  onUserSelect,
  placeholder = "ユーザーID/名",
  disabled = false,
  label,
  noHostMessage = "(まずHostを選択してください)",
}: UserSearchFieldProps) {
  const inputId = useId();
  const [query, setQuery] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const {
    data: searchResult,
    mutateAsync: mutateSearch,
    isPending: isPendingSearch,
  } = useMutation(searchUserInfo);

  const handleQueryChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value.toLowerCase();
    setQuery(value);
    if (!hostId || value.length === 0) return;

    const isId = value.startsWith("u-");
    mutateSearch({
      hostId,
      parameters: {
        user: {
          case: isId ? "userId" : "userName",
          value,
        },
        onlyInContacts: true,
        partialMatch: true,
      },
    });
  };

  const handleUserClick = (user: UserInfo) => {
    onUserSelect(user);
    setQuery("");
    setIsOpen(false);
  };

  const isDisabled = disabled || !hostId;

  return (
    <div className="space-y-1">
      {label && (
        <label
          htmlFor={inputId}
          className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
        >
          {label}
        </label>
      )}
      {!hostId ? (
        <div className="text-sm text-muted-foreground p-2 border rounded-md bg-muted/50">
          {noHostMessage}
        </div>
      ) : (
        <Popover
          open={isOpen && !isDisabled && query.length > 0}
          onOpenChange={setIsOpen}
        >
          <PopoverTrigger asChild>
            <div className="relative">
              <Search className="absolute left-3 top-3 h-4 w-4 text-gray-400" />
              <Input
                id={inputId}
                placeholder={placeholder}
                value={query}
                onChange={handleQueryChange}
                onFocus={() => setIsOpen(true)}
                className="pl-10"
                disabled={disabled}
              />
            </div>
          </PopoverTrigger>
          <PopoverContent
            className="w-[var(--radix-popover-trigger-width)] p-0"
            align="start"
            onOpenAutoFocus={(e) => e.preventDefault()}
          >
            <ScrollBase height="200px">
              <UserList
                data={searchResult?.users || []}
                isLoading={isPendingSearch}
                onUserClick={handleUserClick}
              />
            </ScrollBase>
          </PopoverContent>
        </Popover>
      )}
    </div>
  );
}
