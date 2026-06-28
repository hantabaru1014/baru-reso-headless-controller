import { Outlet, Navigate, useLocation, Link, useMatches } from "react-router";
import { Home, Users, Server, Earth, Clock } from "lucide-react";
import { useMemo } from "react";
import { useAtom } from "jotai";
import { sessionAtom, Session } from "../atoms/sessionAtom";
import { useAuth } from "../hooks/useAuth";
import {
  SidebarProvider,
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarTrigger,
} from "@/components/ui";
import { ThemeToggle } from "@/components/ThemeToggle";
import { cn } from "@/libs/cssUtils";
import { useIsMobile } from "@/hooks/use-mobile";
import { SidebarVersionFooter } from "@/components/SidebarVersionFooter";
import { UserMenuDropdown } from "@/components/UserMenuDropdown";

const navigation = [
  {
    title: "Dashboard",
    href: "/",
    icon: Home,
  },
  {
    title: "Headless Accounts",
    href: "/headlessAccounts",
    icon: Users,
  },
  {
    title: "Hosts",
    href: "/hosts",
    icon: Server,
  },
  {
    title: "Sessions",
    href: "/sessions",
    icon: Earth,
  },
  {
    title: "Scheduled Ops",
    href: "/sessions/scheduled",
    icon: Clock,
  },
];

function AppSidebar() {
  const location = useLocation();

  // 最も長く一致した href のみを active にする。
  // 例: pathname=/sessions/scheduled では /sessions ではなく /sessions/scheduled が選ばれる。
  const activeHref = useMemo(() => {
    const path = location.pathname;
    let best: string | undefined;
    for (const item of navigation) {
      const matched =
        item.href === "/"
          ? path === "/"
          : path === item.href || path.startsWith(item.href + "/");
      if (matched && (!best || item.href.length > best.length)) {
        best = item.href;
      }
    }
    return best;
  }, [location.pathname]);

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {navigation.map((item) => (
                <SidebarMenuItem key={item.href}>
                  <SidebarMenuButton
                    asChild
                    isActive={item.href === activeHref}
                  >
                    <Link to={item.href}>
                      <item.icon />
                      <span>{item.title}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
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

      <ThemeToggle />
      <UserMenuDropdown user={session?.user} signOut={signOut} />
    </header>
  );
}

type RouteHandle = { title?: string };

function usePageTitle(): string | undefined {
  const matches = useMatches();
  for (let i = matches.length - 1; i >= 0; i--) {
    const handle = matches[i].handle as RouteHandle | undefined;
    if (handle?.title) return handle.title;
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
