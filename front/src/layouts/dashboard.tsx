import { useCallback, useRef, useState } from "react";
import { Outlet, Navigate, useLocation, Link } from "react-router";
import { Home, Users, Server, Earth, LogOut, Menu } from "lucide-react";
import { useAtom } from "jotai";
import { sessionAtom, Session } from "../atoms/sessionAtom";
import { useAuth } from "../hooks/useAuth";
import {
  Button,
  Avatar,
  AvatarFallback,
  AvatarImage,
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui";
import { ImperativePanelHandle } from "react-resizable-panels";
import { ThemeToggle } from "@/components/ThemeToggle";

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
];

function Sidebar({ isOpen }: { isOpen: boolean }) {
  const location = useLocation();

  return (
    <div
      className={`bg-background transform transition-transform duration-200 ease-in-out lg:translate-x-0 lg:static lg:inset-0 ${isOpen ? "translate-x-0" : "-translate-x-full"} fixed inset-y-0 left-0 z-50 w-64 lg:relative lg:w-full h-full`}
    >
      <nav className="flex flex-col h-full p-4 space-y-2">
        {navigation.map((item) => {
          const Icon = item.icon;
          const isActive =
            location.pathname === item.href ||
            (item.href !== "/" && location.pathname.startsWith(item.href));

          return (
            <Link
              key={item.href}
              to={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground ${
                isActive
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground"
              }`}
            >
              <Icon className="h-4 w-4" />
              {isOpen && item.title}
            </Link>
          );
        })}
      </nav>
    </div>
  );
}

function Header({
  onMenuClick,
  session,
  signOut,
}: {
  onMenuClick: () => void;
  session?: Session;
  signOut: () => void;
}) {
  return (
    <header className="bg-background border-b px-4 flex items-center justify-between gap-3 lg:px-6">
      <Button variant="ghost" size="icon" onClick={onMenuClick}>
        <Menu className="h-6 w-6" />
      </Button>

      {/* Logo/Branding */}
      <div className="p-3">
        <h1 className="text-xl font-bold">BRHDL</h1>
      </div>

      {/* Spacer */}
      <div className="flex-1" />

      <ThemeToggle />
      {/* User Account */}
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <Avatar className="h-8 w-8">
            <AvatarImage src={session?.user?.image} alt={session?.user?.name} />
            <AvatarFallback>
              {session?.user?.name?.charAt(0) || "U"}
            </AvatarFallback>
          </Avatar>
          <div className="hidden md:block text-sm">
            <div className="font-medium">{session?.user?.name}</div>
            <div className="text-muted-foreground">{session?.user?.email}</div>
          </div>
        </div>
        <Button
          variant="ghost"
          size="icon"
          onClick={signOut}
          title="サインアウト"
        >
          <LogOut className="h-4 w-4" />
        </Button>
      </div>
    </header>
  );
}

export default function Layout() {
  const [session] = useAtom(sessionAtom);
  const location = useLocation();
  const { signOut } = useAuth("/");
  const sidebarRef = useRef<ImperativePanelHandle>(null);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  if (!session) {
    const redirectTo = `/sign-in?callbackUrl=${encodeURIComponent(location.pathname)}`;
    return <Navigate to={redirectTo} replace />;
  }

  const toggleSidebar = useCallback(() => {
    if (sidebarRef.current) {
      if (sidebarRef.current.isCollapsed()) {
        sidebarRef.current.expand();
        setIsSidebarOpen(true);
      } else {
        sidebarRef.current.collapse();
        setIsSidebarOpen(false);
      }
    }
  }, [sidebarRef, setIsSidebarOpen]);

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <Header onMenuClick={toggleSidebar} session={session} signOut={signOut} />
      <ResizablePanelGroup direction="horizontal" className="flex-1">
        <ResizablePanel
          ref={sidebarRef}
          collapsible
          onCollapse={() => setIsSidebarOpen(false)}
          onExpand={() => setIsSidebarOpen(true)}
          collapsedSize={5}
          defaultSize={15}
        >
          <Sidebar isOpen={isSidebarOpen} />
        </ResizablePanel>
        <ResizableHandle />
        <ResizablePanel>
          <main className="p-6">
            <Outlet />
          </main>
        </ResizablePanel>
      </ResizablePanelGroup>
      {/* Overlay for mobile */}
      {isSidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={toggleSidebar}
        />
      )}
    </div>
  );
}
