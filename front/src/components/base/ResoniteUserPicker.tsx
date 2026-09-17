import { useCallback, useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@connectrpc/connect-query";
import { Search, X } from "lucide-react";
import {
  getResoniteUser,
  searchResoniteUsers,
} from "../../../pbgen/hdlctrl/v1/controller-ControllerService_connectquery";
import { useDebounce } from "../../hooks/useDebounce";
import { ResoniteUserIcon } from "../ResoniteUserIcon";
import { Button, Input } from "../ui";
import { Popover, PopoverAnchor, PopoverContent } from "../ui/popover";
import { FieldHeader } from "./FieldHeader";
import { FieldFooter } from "./FieldFooter";
import { ScrollBase } from "./ScrollBase";
import { UserList, UserInfo } from "./UserList";

const SEARCH_DEBOUNCE_MS = 300;

interface ResoniteUserPickerProps {
  value?: UserInfo;
  onChange: (user: UserInfo | undefined) => void;
  label?: string;
  placeholder?: string;
  error?: string;
  disabled?: boolean;
}

/**
 * Resonite Cloud の公開 API でユーザーをインクリメンタルサーチして 1 人選択するフィールド.
 * UserSearchField と違いホスト不要で、コンタクト外のユーザーも引ける.
 */
export function ResoniteUserPicker({
  value,
  onChange,
  label,
  placeholder,
  error,
  disabled = false,
}: ResoniteUserPickerProps) {
  const { t } = useTranslation();
  const inputId = useId();
  const [query, setQuery] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const shouldFocusInputRef = useRef(false);
  // モーダル Dialog 内では body 直下に portal すると Dialog のスクロールロックで
  // 候補リストのホイールスクロールが効かなくなるため、Dialog 内に portal する.
  const [portalContainer, setPortalContainer] = useState<HTMLElement | null>(
    null,
  );
  const anchorRef = useCallback((el: HTMLDivElement | null) => {
    setPortalContainer(el?.closest<HTMLElement>('[role="dialog"]') ?? null);
  }, []);

  // 選択解除で入力欄に戻った時、そのまま再検索できるようにフォーカスする.
  useEffect(() => {
    if (!value && shouldFocusInputRef.current) {
      shouldFocusInputRef.current = false;
      inputRef.current?.focus();
    }
  }, [value]);
  const debouncedQuery = useDebounce(query.trim(), SEARCH_DEBOUNCE_MS);

  // "U-" prefix なら ID 完全一致 lookup (部分一致は Cloud API が非対応)、それ以外は name の部分一致検索.
  const isIdQuery = debouncedQuery.toLowerCase().startsWith("u-");
  const { data: searchResult, isFetching: isFetchingSearch } = useQuery(
    searchResoniteUsers,
    { name: debouncedQuery },
    { enabled: debouncedQuery.length > 0 && !isIdQuery },
  );
  const { data: idLookupResult, isFetching: isFetchingIdLookup } = useQuery(
    getResoniteUser,
    { resoniteId: debouncedQuery },
    // 入力途中の ID は NotFound になるだけなのでリトライしない.
    { enabled: isIdQuery, retry: false },
  );

  const users: UserInfo[] = isIdQuery
    ? idLookupResult
      ? [idLookupResult]
      : []
    : (searchResult?.users ?? []);
  const isLoading =
    query.trim() !== debouncedQuery || isFetchingSearch || isFetchingIdLookup;

  const handleSelect = (user: UserInfo) => {
    onChange({ id: user.id, name: user.name, iconUrl: user.iconUrl });
    setQuery("");
    setIsOpen(false);
  };

  return (
    <div>
      {label && <FieldHeader formId={inputId} label={label} />}
      {value ? (
        <div
          className={`flex items-center justify-between gap-3 rounded-md border p-2 ${error ? "border-destructive" : ""}`}
        >
          <div className="flex min-w-0 items-center gap-3">
            <ResoniteUserIcon
              iconUrl={value.iconUrl}
              alt={t("userList.iconAlt", { name: value.name })}
            />
            <div className="min-w-0">
              <div className="truncate text-sm font-medium">{value.name}</div>
              <div className="text-muted-foreground truncate font-mono text-xs">
                {value.id}
              </div>
            </div>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => {
              shouldFocusInputRef.current = true;
              onChange(undefined);
            }}
            disabled={disabled}
            title={t("resoniteUserPicker.clear")}
          >
            <X />
          </Button>
        </div>
      ) : (
        <Popover
          open={isOpen && !disabled && query.trim().length > 0}
          onOpenChange={setIsOpen}
        >
          <PopoverAnchor asChild>
            <div className="relative" ref={anchorRef}>
              <Search className="absolute left-3 top-3 h-4 w-4 text-gray-400" />
              <Input
                ref={inputRef}
                id={inputId}
                placeholder={placeholder ?? t("resoniteUserPicker.placeholder")}
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setIsOpen(true);
                }}
                onFocus={() => setIsOpen(true)}
                className={`pl-10 ${error ? "border-destructive" : ""}`}
                disabled={disabled}
                autoComplete="off"
              />
            </div>
          </PopoverAnchor>
          <PopoverContent
            className="w-[var(--radix-popover-trigger-width)] p-0"
            align="start"
            container={portalContainer}
            onOpenAutoFocus={(e) => e.preventDefault()}
            onInteractOutside={(e) => {
              // Anchor は Trigger と違い outside 扱いになるので、入力欄クリックで閉じないようにする.
              if (
                e.target instanceof Node &&
                document.getElementById(inputId)?.contains(e.target)
              ) {
                e.preventDefault();
              }
            }}
          >
            <ScrollBase height="200px">
              <UserList
                data={users}
                isLoading={isLoading}
                onUserClick={handleSelect}
                showId
              />
              {!isLoading && users.length === 0 && (
                <p className="text-muted-foreground py-4 text-center text-sm">
                  {t("resoniteUserPicker.noResults")}
                </p>
              )}
            </ScrollBase>
          </PopoverContent>
        </Popover>
      )}
      <FieldFooter error={error} />
    </div>
  );
}
