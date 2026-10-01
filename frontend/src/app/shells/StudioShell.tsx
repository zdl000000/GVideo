import { ArrowUpLeft } from "lucide-react";
import { Link } from "react-router-dom";
import { Brand, GlobalHeader, MobileNavigation, NavigationLinks, ShellContent } from "./ShellChrome";
import type { WorkspaceShellProps } from "./shellTypes";

export function StudioShell(props: WorkspaceShellProps) {
  return (
    <div className="app-shell gv-shell gv-studio-shell">
      <a className="gv-skip-link" href="#main-content">跳到主要内容</a>
      <aside className="gv-studio-rail" aria-label="创作导航">
        <Brand />
        <p className="gv-rail-caption">STUDIO / 创作</p>
        <NavigationLinks user={props.auth.user} studio />
        <div className="gv-rail-bottom"><Link to="/" className="gv-world-link"><ArrowUpLeft size={18} /><span>返回 WATCH</span></Link><p className="gv-studio-note">专注每一次创作。</p></div>
      </aside>
      <GlobalHeader {...props} studio />
      <MobileNavigation {...props} studio />
      <ShellContent />
    </div>
  );
}
