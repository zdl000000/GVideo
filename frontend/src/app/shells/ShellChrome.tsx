import { Suspense, useEffect, useRef } from "react";
import { Bell, Bookmark, Clock3, Compass, Film, Flag, Home, LayoutDashboard, LogIn, LogOut, Menu, Moon, Search, Sun, Upload, Users, X } from "lucide-react";
import { Link, NavLink, Outlet, useLocation } from "react-router-dom";
import mark from "../../assets/gvideo-mark.svg";
import { Avatar } from "../../shared/components/Avatar";
import { LoadingBlock } from "../../shared/components/Feedback";
import { Badge } from "../../shared/ui/Badge";
import { Button, buttonClassName } from "../../shared/ui/Button";
import { IconButton } from "../../shared/ui/IconButton";
import { RouteErrorBoundary } from "../RouteErrorBoundary";
import type { User } from "../../types";
import type { Theme, WorkspaceShellProps } from "./shellTypes";

export function Brand() {
  return <Link to="/" className="brand gv-brand" aria-label="GVideo 首页"><img src={mark} width="36" height="36" alt="" /><span>GVideo<span className="gv-brand-version">3.0</span></span></Link>;
}

export function ThemeButton({ theme, onChange }: { theme: Theme; onChange: () => void }) {
  const label = theme === "light" ? "切换到深色主题" : "切换到浅色主题";
  return <IconButton className="theme-button" label={label} onClick={onChange}>{theme === "light" ? <Moon size={19} /> : <Sun size={19} />}</IconButton>;
}

function AccountMenu({ user, logout }: { user: User; logout: () => Promise<void> }) {
  const ref = useRef<HTMLDetailsElement>(null);
  const location = useLocation();
  useEffect(() => { if (ref.current) ref.current.open = false; }, [location.pathname, location.search]);
  useEffect(() => {
    const closeOutside = (event: PointerEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) ref.current.open = false;
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || !ref.current?.open) return;
      ref.current.open = false;
      ref.current.querySelector("summary")?.focus();
    };
    document.addEventListener("pointerdown", closeOutside);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOutside);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, []);
  return <details className="gv-account-menu" ref={ref}>
    <summary className="user-chip" aria-label={`${user.username} 的账户菜单`}><Avatar username={user.username} src={user.avatar_url} size="small" /><span>{user.username}</span></summary>
    <nav aria-label="账户导航" className="gv-account-panel">
      <Link to={`/users/${user.id}`}>个人空间</Link><Link to="/favorites">我的收藏</Link><Link to="/creator">创作中心</Link><Link to="/me/videos">投稿管理</Link>
      {user.is_admin && <Link to="/admin/reports">举报审核</Link>}
      <Button variant="ghost" onClick={logout}><LogOut size={16} />退出登录</Button>
    </nav>
  </details>;
}

export function GlobalHeader({ auth, theme, onThemeChange, navigation, studio = false }: WorkspaceShellProps & { studio?: boolean }) {
  return <header className="topbar gv-topbar">
    <div className="gv-header-identity"><Brand /><span className="gv-workspace-label">{studio ? "CREATOR STUDIO" : "WATCH · CREATE · SHARE"}</span></div>
    <form className="global-search" onSubmit={(event) => { event.preventDefault(); navigation.search(); }}>
      <Search size={17} aria-hidden="true" /><input value={navigation.query} onChange={(event) => navigation.setQuery(event.target.value)} placeholder="搜索视频、创作者" aria-label="搜索" /><IconButton type="submit" label="搜索"><Search size={17} /></IconButton>
    </form>
    <nav className="desktop-actions" aria-label="用户操作">
      <ThemeButton theme={theme} onChange={onThemeChange} />
      {auth.user ? <>
        <Link to="/notifications" className="gv-icon-button notification-button" title="通知中心" aria-label={`通知中心，${navigation.unreadNotifications} 条未读`}><Bell size={19} />{navigation.unreadNotifications > 0 && <Badge tone="brand" aria-hidden="true">{navigation.unreadNotifications > 99 ? "99+" : navigation.unreadNotifications}</Badge>}</Link>
        <AccountMenu user={auth.user} logout={navigation.logout} />
        <Link to="/upload" className={buttonClassName("primary", "gv-publish-action")}><Upload size={16} />投稿</Link>
      </> : <Link to="/auth" className={buttonClassName()}><LogIn size={16} />登录</Link>}
    </nav>
    <IconButton ref={navigation.menuButtonRef} className="mobile-menu-button" label={navigation.menuOpen ? "关闭菜单" : "打开菜单"} onClick={() => navigation.setMenuOpen(!navigation.menuOpen)} aria-expanded={navigation.menuOpen} aria-controls="mobile-sidebar">{navigation.menuOpen ? <X size={21} /> : <Menu size={21} />}</IconButton>
  </header>;
}

export function NavigationLinks({ user, studio = false, mobile = false }: { user: User | null; studio?: boolean; mobile?: boolean }) {
  const link = (to: string, label: string, Icon: typeof Home, end = false) => <NavLink key={to} to={to} end={end} title={label} className={({ isActive }) => `gv-nav-link ${isActive ? "active" : ""}`}><Icon size={19} aria-hidden="true" /><span>{label}</span></NavLink>;
  return <nav className="gv-rail-navigation" aria-label={studio ? "Studio 导航" : "主导航"}>
    {studio ? <>{link("/creator", "创作概览", LayoutDashboard)}{link("/me/videos", "投稿管理", Film)}{link("/upload", "发布视频", Upload)}{user?.is_admin && link("/admin/reports", "举报审核", Flag)}</> : <>{link("/", "首页", Home, true)}{link("/popular", "热门发现", Compass)}{link("/latest", "最新发布", Clock3)}{user && link("/following", "关注动态", Users)}{user && link("/favorites", "我的收藏", Bookmark)}</>}
    {mobile && <><span className="gv-nav-divider" />{studio ? link("/", "返回观看", Home, true) : <>{link(user ? "/creator" : "/auth?next=/creator", "创作中心", LayoutDashboard)}{link(user ? "/me/videos" : "/auth?next=/me/videos", "投稿管理", Film)}{link(user ? "/upload" : "/auth?next=/upload", "发布视频", Upload)}{user?.is_admin && link("/admin/reports", "举报审核", Flag)}</>}{user && link("/notifications", "通知中心", Bell)}</>}
  </nav>;
}

export function MobileNavigation({ auth, theme, onThemeChange, navigation, studio = false }: WorkspaceShellProps & { studio?: boolean }) {
  const open = navigation.isMobile && navigation.menuOpen;
  return <>
    <div className={`gv-drawer-scrim ${open ? "open" : ""}`} onClick={() => navigation.setMenuOpen(false)} aria-hidden="true" />
    <aside ref={navigation.drawerRef} id="mobile-sidebar" className={`sidebar gv-drawer ${open ? "open" : ""}`} role={open ? "dialog" : undefined} aria-modal={open ? true : undefined} aria-label="导航菜单" aria-hidden={!open} inert={!open ? true : undefined}>
      <div className="gv-drawer-heading"><Brand /><IconButton label="关闭菜单" onClick={() => navigation.setMenuOpen(false)}><X size={20} /></IconButton></div>
      <p className="gv-rail-caption">{studio ? "STUDIO / 创作" : "WATCH / 观看"}</p>
      <NavigationLinks user={auth.user} studio={studio} mobile />
      <div className="mobile-account">
        <Button variant="ghost" className="theme-row" onClick={onThemeChange} aria-label={theme === "light" ? "切换到深色主题" : "切换到浅色主题"}>{theme === "light" ? <Moon size={18} /> : <Sun size={18} />}{theme === "light" ? "深色主题" : "浅色主题"}</Button>
        {auth.user ? <><Link to={`/users/${auth.user.id}`} className="user-chip"><Avatar username={auth.user.username} src={auth.user.avatar_url} size="small" /><span>{auth.user.username}</span></Link><Button variant="secondary" onClick={navigation.logout}><LogOut size={17} />退出登录</Button></> : <Link to="/auth" className={buttonClassName()}><LogIn size={17} />登录或注册</Link>}
      </div>
    </aside>
  </>;
}

export function ShellContent({ auth = false }: { auth?: boolean }) {
  const location = useLocation();
  return <main id="main-content" className={auth ? "auth-main" : "main-content gv-main"} tabIndex={-1}><RouteErrorBoundary key={location.key}><Suspense fallback={<LoadingBlock label="正在加载页面" />}><Outlet /></Suspense></RouteErrorBoundary></main>;
}
