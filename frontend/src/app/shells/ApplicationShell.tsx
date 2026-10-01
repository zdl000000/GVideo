import { useLocation } from "react-router-dom";
import { AuthShell } from "./AuthShell";
import { StudioShell } from "./StudioShell";
import { WatchShell } from "./WatchShell";
import { useShellNavigation } from "./useShellNavigation";
import type { ShellProps } from "./shellTypes";

const studioPaths = new Set(["/creator", "/me/videos", "/upload", "/admin/reports"]);

export function ApplicationShell(props: ShellProps) {
  const location = useLocation();
  const navigation = useShellNavigation(props);
  if (location.pathname.replace(/\/$/, "") === "/auth") return <AuthShell {...props} />;
  const Shell = studioPaths.has(location.pathname.replace(/\/$/, "")) ? StudioShell : WatchShell;
  return <Shell {...props} navigation={navigation} />;
}
