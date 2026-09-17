import { Outlet, Navigate, useLocation, Link, useMatches } from "react-router";
import {
  Home,
  Users,
  Server,
  Earth,
  Clock,
  History,
  UsersRound,
  ShieldCheck,
  ChevronRight,
} from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useAtom } from "jotai";
import { sessionAtom, Session } from "../atoms/sessionAtom";
import { useAuth } from "../hooks/useAuth";
import { usePermissions } from "../hooks/usePermissions";
import { PERMISSION_KEYS } from "../libs/permissionUtils";
import {
  SidebarProvider,
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarMenuSub,
  SidebarMenuSubItem,
  SidebarMenuSubButton,
  SidebarTrigger,
  useSidebar,
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuItem,
} from "@/components/ui";
import { ThemeToggle } from "@/components/ThemeToggle";
import { cn } from "@/libs/cssUtils";
import { useIsMobile } from "@/hooks/use-mobile";
import { SidebarVersionFooter } from "@/components/SidebarVersionFooter";
import { UserMenuDropdown } from "@/components/UserMenuDropdown";
import { GroupSwitcher } from "@/components/base/GroupSwitcher";
import { ADMIN_SECTIONS } from "../pages/admin/sections";

type Perms = ReturnType<typeof usePermissions>;

type NavLeaf = {
  titleKey: string;
  href: string;
  /** undefined のとき常時表示. 関数のとき true を返したものだけ表示する. */
  visible?: (perms: Perms) => boolean;
};

type NavLink = NavLeaf & { icon: typeof Home };

/** 折り畳みでサブ項目を持つ項目. 自身は遷移先を持たず, 表示できる children が無ければ非表示. */
type NavGroup = {
  titleKey: string;
  icon: typeof Home;
  children: NavLeaf[];
};

type NavItem = NavLink | NavGroup;

const isNavGroup = (item: NavItem): item is NavGroup => "children" in item;

const navigation: NavItem[] = [
  {
    titleKey: "routes.dashboard",
    href: "/",
    icon: Home,
  },
  {
    titleKey: "routes.headlessAccounts",
    href: "/headlessAccounts",
    icon: Users,
    visible: (p) =>
      p.groupsWithPermission(PERMISSION_KEYS.ACCOUNT_READ).length > 0 ||
      p.hasSystemPermission(PERMISSION_KEYS.SYSTEM_GROUP_MANAGE),
  },
  {
    titleKey: "routes.hosts",
    href: "/hosts",
    icon: Server,
    visible: (p) =>
      p.groupsWithPermission(PERMISSION_KEYS.HOST_READ).length > 0 ||
      p.hasSystemPermission(PERMISSION_KEYS.SYSTEM_GROUP_MANAGE),
  },
  {
    titleKey: "routes.sessions",
    href: "/sessions",
    icon: Earth,
    visible: (p) =>
      p.groupsWithPermission(PERMISSION_KEYS.SESSION_READ).length > 0 ||
      p.hasSystemPermission(PERMISSION_KEYS.SYSTEM_GROUP_MANAGE),
  },
  {
    titleKey: "routes.scheduledOps",
    href: "/sessions/scheduled",
    icon: Clock,
    visible: (p) =>
      p.groupsWithPermission(PERMISSION_KEYS.SESSION_WRITE).length > 0 ||
      p.hasSystemPermission(PERMISSION_KEYS.SYSTEM_GROUP_MANAGE),
  },
  {
    titleKey: "routes.asyncJobs",
    href: "/async-jobs",
    icon: History,
  },
  {
    titleKey: "routes.groups",
    href: "/groups",
    icon: UsersRound,
  },
  {
    titleKey: "routes.admin",
    icon: ShieldCheck,
    children: ADMIN_SECTIONS.map((s) => ({
      titleKey: s.titleKey,
      href: s.href,
      visible: (p) => s.permissions.some((key) => p.hasSystemPermission(key)),
    })),
  },
];

function NavGroupItem({
  item,
  activeHref,
}: {
  item: NavGroup;
  activeHref: string | undefined;
}) {
  const { t } = useTranslation();
  const { state, isMobile } = useSidebar();
  const groupActive = item.children.some((c) => c.href === activeHref);

  // 配下の画面に入ったときは自動で開く. それ以外はユーザーの開閉操作を保持する.
  const [open, setOpen] = useState(groupActive);
  const [prevGroupActive, setPrevGroupActive] = useState(groupActive);
  if (groupActive !== prevGroupActive) {
    setPrevGroupActive(groupActive);
    if (groupActive) setOpen(true);
  }

  // アイコンのみ表示中はサブ項目を展開できないので, ドロップダウンで各画面へ直接飛べるようにする.
  if (state === "collapsed" && !isMobile) {
    return (
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton isActive={groupActive}>
              <item.icon />
              <span>{t(item.titleKey)}</span>
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="right" align="start">
            <DropdownMenuLabel>{t(item.titleKey)}</DropdownMenuLabel>
            {item.children.map((child) => (
              <DropdownMenuItem key={child.href} asChild>
                <Link to={child.href}>{t(child.titleKey)}</Link>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    );
  }

  return (
    <Collapsible
      asChild
      open={open}
      onOpenChange={setOpen}
      className="group/collapsible"
    >
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton isActive={groupActive && !open}>
            <item.icon />
            <span>{t(item.titleKey)}</span>
            <ChevronRight className="ml-auto transition-transform group-data-[state=open]/collapsible:rotate-90" />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            {item.children.map((child) => (
              <SidebarMenuSubItem key={child.href}>
                <SidebarMenuSubButton
                  asChild
                  isActive={child.href === activeHref}
                >
                  <Link to={child.href}>
                    <span>{t(child.titleKey)}</span>
                  </Link>
                </SidebarMenuSubButton>
              </SidebarMenuSubItem>
            ))}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
}

function AppSidebar() {
  const { t } = useTranslation();
  const location = useLocation();
  const perms = usePermissions();
  const visibleNavigation = useMemo(() => {
    const isVisible = (n: NavLeaf) => (n.visible ? n.visible(perms) : true);
    return navigation.flatMap<NavItem>((item) => {
      if (!isNavGroup(item)) return isVisible(item) ? [item] : [];
      const children = item.children.filter(isVisible);
      return children.length > 0 ? [{ ...item, children }] : [];
    });
  }, [perms]);

  // 最も長く一致した href のみを active にする。
  // 例: pathname=/sessions/scheduled では /sessions ではなく /sessions/scheduled が選ばれる。
  const activeHref = useMemo(() => {
    const path = location.pathname;
    let best: string | undefined;
    const leaves = visibleNavigation.flatMap<NavLeaf>((item) =>
      isNavGroup(item) ? item.children : [item],
    );
    for (const item of leaves) {
      const matched =
        item.href === "/"
          ? path === "/"
          : path === item.href || path.startsWith(item.href + "/");
      if (matched && (!best || item.href.length > best.length)) {
        best = item.href;
      }
    }
    return best;
  }, [location.pathname, visibleNavigation]);

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {visibleNavigation.map((item) =>
                isNavGroup(item) ? (
                  <NavGroupItem
                    key={item.titleKey}
                    item={item}
                    activeHref={activeHref}
                  />
                ) : (
                  <SidebarMenuItem key={item.href}>
                    <SidebarMenuButton
                      asChild
                      isActive={item.href === activeHref}
                    >
                      <Link to={item.href}>
                        <item.icon />
                        <span>{t(item.titleKey)}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ),
              )}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarVersionFooter version={__APP_VERSION__} />
    </Sidebar>
  );
}

function Header({
  session,
  signOut,
  title,
}: {
  session?: Session;
  signOut: () => void;
  title?: string;
}) {
  return (
    <header className="bg-background border-b px-4 flex items-center justify-between gap-3 lg:px-6">
      <SidebarTrigger />

      <div className="p-3">
        <h1 className="text-xl font-bold">{title ?? ""}</h1>
      </div>

      {/* Spacer */}
      <div className="flex-1" />

      <GroupSwitcher />
      <ThemeToggle />
      <UserMenuDropdown user={session?.user} signOut={signOut} />
    </header>
  );
}

type RouteHandle = { titleKey?: string };

function usePageTitle(): string | undefined {
  const { t } = useTranslation();
  const matches = useMatches();
  for (let i = matches.length - 1; i >= 0; i--) {
    const handle = matches[i].handle as RouteHandle | undefined;
    if (handle?.titleKey) return t(handle.titleKey);
  }
  return undefined;
}

export default function Layout() {
  const [session] = useAtom(sessionAtom);
  const location = useLocation();
  const { signOut } = useAuth("/");
  const isMobile = useIsMobile();
  const title = usePageTitle();

  if (!session) {
    const redirectTo = `/sign-in?callbackUrl=${encodeURIComponent(location.pathname + location.search)}`;
    return <Navigate to={redirectTo} replace />;
  }

  return (
    <SidebarProvider>
      <AppSidebar />
      <div
        data-slot="sidebar-inset"
        className={cn(
          "bg-background relative flex w-full flex-1 flex-col",
          "md:peer-data-[variant=inset]:m-2 md:peer-data-[variant=inset]:ml-0 md:peer-data-[variant=inset]:rounded-xl md:peer-data-[variant=inset]:shadow-sm md:peer-data-[variant=inset]:peer-data-[state=collapsed]:ml-2",
        )}
      >
        <Header session={session} signOut={signOut} title={title} />
        <main className={cn(isMobile ? "p-1" : "p-6")}>
          <Outlet />
        </main>
      </div>
    </SidebarProvider>
  );
}
