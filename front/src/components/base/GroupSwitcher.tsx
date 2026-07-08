import { useQuery } from "@connectrpc/connect-query";
import { useAtom } from "jotai";
import { useEffect, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Check, ChevronDown, Users } from "lucide-react";
import { toast } from "sonner";
import { Button } from "../ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";
import { currentGroupIdAtom } from "../../atoms/currentGroupAtom";
import { listGroups } from "../../../pbgen/hdlctrl/v1/permission-GroupService_connectquery";
import { GroupType } from "../../../pbgen/hdlctrl/v1/permission_pb";
import { groupTypeToLabel } from "../../libs/permissionUtils";
import { cn } from "../../libs/cssUtils";

/**
 * ヘッダーに置く「コンテキストグループ」セレクタ.
 *
 * 選択結果は `currentGroupIdAtom` に保持され、各リスト画面が
 * その値を `group_id` クエリパラメータに使って一覧をフィルタする.
 *
 * `listGroups` はバックエンドで以下のように出し分けされる:
 *  - `system:group.list` 保持者: 全グループ
 *  - それ以外: 自分が所属するグループのみ
 *
 * よって追加のロール判定は不要で、返ってきたグループをそのまま並べれば良い.
 */
export function GroupSwitcher() {
  const { t } = useTranslation();
  const [currentGroupId, setCurrentGroupId] = useAtom(currentGroupIdAtom);
  const { data, isFetched } = useQuery(listGroups, {});

  const groups = useMemo(() => {
    const list = [...(data?.groups ?? [])];
    list.sort((a, b) => {
      if (a.type !== b.type) {
        // personal -> normal -> system の順
        const order = (t: GroupType) =>
          t === GroupType.PERSONAL ? 0 : t === GroupType.SYSTEM ? 2 : 1;
        return order(a.type) - order(b.type);
      }
      return a.name.localeCompare(b.name);
    });
    return list;
  }, [data?.groups]);

  const selectedGroup = useMemo(
    () =>
      currentGroupId ? groups.find((g) => g.id === currentGroupId) : undefined,
    [currentGroupId, groups],
  );

  // 選択中グループが消失した場合 (脱退・削除など) は null にフォールバック.
  useEffect(() => {
    if (!isFetched || !currentGroupId) return;
    if (!groups.some((g) => g.id === currentGroupId)) {
      setCurrentGroupId(null);
      toast.warning(t("groupSwitcher.selectedGroupNotFound"));
    }
  }, [isFetched, currentGroupId, groups, setCurrentGroupId, t]);

  const triggerLabel = currentGroupId
    ? (selectedGroup?.name ?? t("groupSwitcher.unknownGroup"))
    : t("groupSwitcher.allGroups");

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="max-w-[14rem] gap-1 px-2"
          title={t("groupSwitcher.targetGroupLabel", { name: triggerLabel })}
        >
          <Users className="h-4 w-4 shrink-0" />
          <span className="text-muted-foreground hidden md:inline">
            {t("groupSwitcher.groupPrefix")}
          </span>
          <span className="truncate">{triggerLabel}</span>
          <ChevronDown className="h-4 w-4 shrink-0 opacity-60" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-[16rem]">
        <DropdownMenuLabel>{t("groupSwitcher.targetGroup")}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => setCurrentGroupId(null)}>
          <Check
            className={cn(
              "h-4 w-4",
              currentGroupId === null ? "opacity-100" : "opacity-0",
            )}
          />
          <span>{t("groupSwitcher.allGroups")}</span>
          <span className="text-muted-foreground ml-auto text-xs">
            {t("groupSwitcher.allAccessible")}
          </span>
        </DropdownMenuItem>
        {groups.length > 0 && <DropdownMenuSeparator />}
        {groups.map((g) => (
          <DropdownMenuItem key={g.id} onClick={() => setCurrentGroupId(g.id)}>
            <Check
              className={cn(
                "h-4 w-4",
                currentGroupId === g.id ? "opacity-100" : "opacity-0",
              )}
            />
            <span className="truncate">{g.name}</span>
            <span className="text-muted-foreground ml-auto text-xs shrink-0">
              {groupTypeToLabel(g.type)}
            </span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
