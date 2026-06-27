import { SidebarFooter } from "@/components/ui";

const GITHUB_REPO_URL =
  "https://github.com/hantabaru1014/baru-reso-headless-controller";

function getVersionLink(version: string): string {
  if (version === "dev") return GITHUB_REPO_URL;
  if (/^v\d/.test(version)) return `${GITHUB_REPO_URL}/releases/tag/${version}`;
  return `${GITHUB_REPO_URL}/commit/${version}`;
}

export function SidebarVersionFooter({ version }: { version: string }) {
  const label = `BRHDL ${version}`;
  return (
    <SidebarFooter className="group-data-[collapsible=icon]:hidden">
      <a
        href={getVersionLink(version)}
        target="_blank"
        rel="noopener noreferrer"
        className="text-sidebar-foreground/70 hover:text-sidebar-foreground truncate px-2 text-xs"
        title={label}
      >
        {label}
      </a>
    </SidebarFooter>
  );
}
