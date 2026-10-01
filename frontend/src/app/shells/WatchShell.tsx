import { ArrowUpRight } from "lucide-react";
import { Link } from "react-router-dom";
import { Brand, GlobalHeader, MobileNavigation, NavigationLinks, ShellContent } from "./ShellChrome";
import type { WorkspaceShellProps } from "./shellTypes";

export function WatchShell(props: WorkspaceShellProps) {
  return (
    <div className="app-shell gv-shell gv-watch-shell">
      <a className="gv-skip-link" href="#main-content">跳到主要内容</a>
      <aside className="gv-brand-rail" aria-label="观看导航">
        <Brand />
        <p className="gv-rail-caption">WATCH / 观看</p>
        <NavigationLinks user={props.auth.user} />
        <div className="gv-rail-bottom">
          <Link to={props.auth.user ? "/creator" : "/auth?next=/creator"} className="gv-world-link" title="进入创作中心"><ArrowUpRight size={18} /><span>进入 STUDIO</span></Link>
          <p className="gv-brand-signature">WATCH<br />CREATE<br />SHARE<span>让内容连接更大的世界</span></p>
        </div>
      </aside>
      <GlobalHeader {...props} />
      <MobileNavigation {...props} />
      <ShellContent />
    </div>
  );
}
