import { FormEvent, useEffect, useLayoutEffect, useRef, useState } from "react";
import {
  Bell,
  Bookmark,
  Clock3,
  Compass,
  Film,
  Flag,
  Home,
  LayoutDashboard,
  LogIn,
  LogOut,
  Menu,
  Moon,
  Play,
  Search,
  Sparkles,
  Sun,
  Upload,
  Users,
  X
} from "lucide-react";
import { Link, NavLink, Navigate, Outlet, Route, Routes, useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { ApiError, api, setCSRFToken } from "../shared/api/client";
import { AuthPage } from "../features/auth/AuthPage";
import { CreatorDashboard } from "../features/creator/CreatorDashboard";
import { CreatorPage } from "../features/creator/CreatorPage";
import { NotificationCenterPage } from "../features/notifications/NotificationCenterPage";
import { AdminReportsPage } from "../features/moderation/AdminReportsPage";
import { FavoritesPage } from "../features/videos/FavoritesPage";
import { FollowingPage } from "../features/videos/FollowingPage";
import { HomePage } from "../features/videos/HomePage";
import { LatestPage } from "../features/videos/LatestPage";
import { MyVideosPage } from "../features/videos/MyVideosPage";
import { PopularPage } from "../features/videos/PopularPage";
import { UploadPage } from "../features/upload/UploadPage";
import { VideoPage } from "../features/watch/VideoPage";
import { Avatar } from "../shared/components/Avatar";
import { LoadingBlock, NotFound } from "../shared/components/Feedback";
import type { AuthPayload, User } from "../types";

type Theme = "light" | "dark";

interface AuthState {
  user: User | null;
  loading: boolean;
}

function App() {
  const [auth, setAuth] = useState<AuthState>({ user: null, loading: true });
  const authRequestRef = useRef(0);
  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem("gvideo-theme");
    if (saved === "light" || saved === "dark") return saved;
    return "light";
  });

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    document.querySelector('meta[name="theme-color"]')?.setAttribute("content", theme === "dark" ? "#15171b" : "#f7f8fa");
    localStorage.setItem("gvideo-theme", theme);
  }, [theme]);

  useEffect(() => {
    const requestID = ++authRequestRef.current;
    api.me()
      .then((payload) => {
        if (requestID === authRequestRef.current) applyAuth(payload);
      })
      .catch((error) => {
        if (requestID !== authRequestRef.current) return;
        if (!(error instanceof ApiError && error.status === 401)) console.error(error);
        clearAuth();
      });
  }, []);

  const applyAuth = (payload: AuthPayload) => {
    authRequestRef.current += 1;
    setCSRFToken(payload.csrf_token);
    setAuth({ user: payload.user, loading: false });
  };

  const clearAuth = () => {
    authRequestRef.current += 1;
    setCSRFToken("");
    setAuth({ user: null, loading: false });
  };

  useEffect(() => {
    window.addEventListener("gvideo-auth-expired", clearAuth);
    return () => window.removeEventListener("gvideo-auth-expired", clearAuth);
  }, []);

  const updateAuthUser = (user: User) => setAuth((current) => ({ ...current, user }));

  return (
    <Routes>
      <Route element={<Shell auth={auth} onLogout={clearAuth} theme={theme} onThemeChange={() => setTheme((current) => current === "light" ? "dark" : "light")} />}>
        <Route index element={<HomeRoute />} />
        <Route path="latest" element={<LatestPage />} />
        <Route path="popular" element={<PopularPage />} />
        <Route path="following" element={<Protected auth={auth}><FollowingPage /></Protected>} />
        <Route path="favorites" element={<Protected auth={auth}><FavoritesPage /></Protected>} />
        <Route path="users/:id" element={<CreatorPage user={auth.user} onUserUpdated={updateAuthUser} />} />
        <Route path="video/:id" element={<VideoPage user={auth.user} />} />
        <Route path="auth" element={auth.user ? <Navigate to="/" replace /> : <AuthPage onAuth={applyAuth} />} />
        <Route path="upload" element={<Protected auth={auth}><UploadPage /></Protected>} />
        <Route path="me/videos" element={<Protected auth={auth}><MyVideosPage /></Protected>} />
        <Route path="creator" element={<Protected auth={auth}><CreatorDashboard user={auth.user!} /></Protected>} />
        <Route path="notifications" element={<Protected auth={auth}><NotificationCenterPage /></Protected>} />
        <Route path="admin/reports" element={<AdminProtected auth={auth}><AdminReportsPage /></AdminProtected>} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}

function Protected({ auth, children }: { auth: AuthState; children: React.ReactNode }) {
  const location = useLocation();
  if (auth.loading) return <LoadingBlock label="正在确认登录状态" />;
  if (!auth.user) return <Navigate to={`/auth?next=${encodeURIComponent(`${location.pathname}${location.search}`)}`} replace />;
  return children;
}

function AdminProtected({ auth, children }: { auth: AuthState; children: React.ReactNode }) {
  const location = useLocation();
  if (auth.loading) return <LoadingBlock label="正在确认管理员权限" />;
  if (!auth.user) return <Navigate to={`/auth?next=${encodeURIComponent(`${location.pathname}${location.search}`)}`} replace />;
  if (!auth.user.is_admin) return <Navigate to="/" replace />;
  return children;
}

function Shell({ auth, onLogout, theme, onThemeChange }: { auth: AuthState; onLogout: () => void; theme: Theme; onThemeChange: () => void }) {
  const navigate = useNavigate();
  const location = useLocation();
  const [menuOpen, setMenuOpen] = useState(false);
  const [query, setQuery] = useState("");
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const sidebarRef = useRef<HTMLElement>(null);
  const wasMenuOpenRef = useRef(false);
  const [isMobileSidebar, setIsMobileSidebar] = useState(() => window.matchMedia("(max-width: 920px)").matches);
  const [unreadNotifications, setUnreadNotifications] = useState(0);

  useEffect(() => setMenuOpen(false), [location.pathname, location.search]);
  useEffect(() => {
    if (!auth.user) {
      setUnreadNotifications(0);
      return;
    }
    const refresh = () => {
      const params = new URLSearchParams({ page_size: "1" });
      api.notifications(params).then((result) => setUnreadNotifications(result.unread_count)).catch(() => undefined);
    };
    refresh();
    window.addEventListener("gvideo-notifications-changed", refresh);
    return () => window.removeEventListener("gvideo-notifications-changed", refresh);
  }, [auth.user, location.pathname]);
  useEffect(() => {
    const mediaQuery = window.matchMedia("(max-width: 920px)");
    const update = () => setIsMobileSidebar(mediaQuery.matches);
    update();
    mediaQuery.addEventListener("change", update);
    return () => mediaQuery.removeEventListener("change", update);
  }, []);
  useLayoutEffect(() => {
    if (!menuOpen) {
      if (wasMenuOpenRef.current) menuButtonRef.current?.focus();
      wasMenuOpenRef.current = false;
      return;
    }
    wasMenuOpenRef.current = true;
    const first = sidebarRef.current?.querySelector<HTMLElement>("a,button,input,select,textarea,[tabindex]:not([tabindex='-1'])");
    first?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenuOpen(false);
        return;
      }
      if (event.key !== "Tab" || !sidebarRef.current) return;
      const focusable = Array.from(sidebarRef.current.querySelectorAll<HTMLElement>("a,button,input,select,textarea,[tabindex]:not([tabindex='-1'])"));
      if (!focusable.length) return;
      const firstFocusable = focusable[0];
      const lastFocusable = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === firstFocusable) {
        event.preventDefault();
        lastFocusable.focus();
      } else if (!event.shiftKey && document.activeElement === lastFocusable) {
        event.preventDefault();
        firstFocusable.focus();
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [menuOpen]);

  const submitSearch = (event: FormEvent) => {
    event.preventDefault();
    navigate(query.trim() ? `/?q=${encodeURIComponent(query.trim())}` : "/");
  };

  const logout = async () => {
    await api.logout().catch(console.error);
    setMenuOpen(false);
    onLogout();
    navigate("/");
  };

  if (location.pathname === "/auth") {
    return (
      <div className="auth-shell">
        <header className="auth-topbar">
          <Link to="/" className="brand" aria-label="GVideo 首页">
            <span className="brand-mark"><Play size={17} fill="currentColor" /></span>
            <span>GVideo</span>
          </Link>
          <ThemeButton theme={theme} onChange={onThemeChange} />
        </header>
        <main className="auth-main">
          <Outlet />
        </main>
      </div>
    );
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <Link to="/" className="brand" aria-label="GVideo 首页">
          <span className="brand-mark"><Play size={17} fill="currentColor" /></span>
          <span>GVideo</span>
        </Link>
        <form className="global-search" onSubmit={submitSearch}>
          <Search size={18} />
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索视频、创作者" aria-label="搜索" />
          <button type="submit" className="icon-button" title="搜索"><Search size={18} /></button>
        </form>
        <nav className="desktop-actions" aria-label="用户操作">
          <ThemeButton theme={theme} onChange={onThemeChange} />
          {auth.user ? (
            <>
              <Link to="/notifications" className="icon-button notification-button" title="通知中心" aria-label={`通知中心，${unreadNotifications} 条未读`}>
                <Bell size={19} />
                {unreadNotifications > 0 && <span>{unreadNotifications > 99 ? "99+" : unreadNotifications}</span>}
              </Link>
              <Link to={`/users/${auth.user.id}`} className="user-chip"><Avatar username={auth.user.username} src={auth.user.avatar_url} size="small" />{auth.user.username}</Link>
              <Link to="/upload" className="primary-button compact"><Upload size={17} />投稿</Link>
              <button className="icon-button" onClick={logout} title="退出登录"><LogOut size={19} /></button>
            </>
          ) : (
            <Link to="/auth" className="primary-button compact"><LogIn size={17} />登录</Link>
          )}
        </nav>
        <button ref={menuButtonRef} className="mobile-menu-button icon-button" onClick={() => setMenuOpen((open) => !open)} title={menuOpen ? "关闭菜单" : "打开菜单"} aria-label={menuOpen ? "关闭菜单" : "打开菜单"} aria-expanded={menuOpen} aria-controls="mobile-sidebar">
          {menuOpen ? <X size={21} /> : <Menu size={21} />}
        </button>
      </header>

      <nav className="channel-bar" aria-label="频道导航">
        <div className="channel-bar-inner">
          <NavLink to="/" end><Home size={16} />首页</NavLink>
          <NavLink to="/latest"><Clock3 size={16} />最新</NavLink>
          <NavLink to="/popular"><Compass size={16} />热门</NavLink>
          {auth.user && <NavLink to="/following"><Users size={16} />关注</NavLink>}
          {auth.user && <NavLink to="/favorites"><Bookmark size={16} />收藏</NavLink>}
          <span className="channel-divider" aria-hidden="true" />
          {auth.user && <NavLink to="/creator"><LayoutDashboard size={16} />创作中心</NavLink>}
          <NavLink to={auth.user ? "/me/videos" : "/auth?next=/me/videos"}><Film size={16} />投稿管理</NavLink>
          {auth.user?.is_admin && <NavLink to="/admin/reports"><Flag size={16} />举报审核</NavLink>}
        </div>
      </nav>

      <button className={`sidebar-scrim ${menuOpen ? "open" : ""}`} onClick={() => setMenuOpen(false)} aria-label="关闭菜单" tabIndex={menuOpen ? 0 : -1} />

      <aside ref={sidebarRef} id="mobile-sidebar" className={`sidebar ${menuOpen ? "open" : ""}`} aria-hidden={isMobileSidebar && !menuOpen} inert={isMobileSidebar && !menuOpen ? true : undefined}>
        <nav aria-label="主导航">
          <NavItem to="/" icon={<Home size={19} />} label="首页" end />
          <NavItem to="/latest" icon={<Clock3 size={19} />} label="最新发布" />
          <NavItem to="/popular" icon={<Compass size={19} />} label="热门发现" />
          {auth.user && <NavItem to="/following" icon={<Users size={19} />} label="关注动态" />}
          {auth.user && <NavItem to="/favorites" icon={<Bookmark size={19} />} label="我的收藏" />}
          {auth.user && <NavItem to="/creator" icon={<LayoutDashboard size={19} />} label="创作者中心" />}
          {auth.user && <NavItem to="/notifications" icon={<Bell size={19} />} label="通知中心" />}
          {auth.user?.is_admin && <NavItem to="/admin/reports" icon={<Flag size={19} />} label="举报审核" />}
          <NavItem to={auth.user ? "/me/videos" : "/auth?next=/me/videos"} icon={<Film size={19} />} label="我的投稿" />
          <NavItem to={auth.user ? "/upload" : "/auth?next=/upload"} icon={<Upload size={19} />} label="发布视频" />
        </nav>
        <div className="mobile-account">
          <button className="theme-row" onClick={onThemeChange} aria-label={theme === "light" ? "切换到深色主题" : "切换到浅色主题"}>
            <span>{theme === "light" ? <Moon size={18} /> : <Sun size={18} />}{theme === "light" ? "深色主题" : "浅色主题"}</span>
            <span className={`theme-switch ${theme === "dark" ? "active" : ""}`} aria-hidden="true"><i /></span>
          </button>
          {auth.user ? (
            <>
              <Link to={`/users/${auth.user.id}`} className="user-chip"><Avatar username={auth.user.username} src={auth.user.avatar_url} size="small" /><b>{auth.user.username}</b></Link>
              <button className="secondary-button" onClick={logout}><LogOut size={17} />退出登录</button>
            </>
          ) : (
            <Link to="/auth" className="primary-button"><LogIn size={17} />登录或注册</Link>
          )}
        </div>
        <div className="sidebar-note">
          <span><Sparkles size={16} />创作提示</span>
          <p>标题清楚、封面准确，更容易让观众找到你。</p>
        </div>
      </aside>

      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}

function ThemeButton({ theme, onChange }: { theme: Theme; onChange: () => void }) {
  const nextLabel = theme === "light" ? "切换到深色主题" : "切换到浅色主题";
  return <button className="icon-button theme-button" onClick={onChange} title={nextLabel} aria-label={nextLabel}>{theme === "light" ? <Moon size={19} /> : <Sun size={19} />}</button>;
}

function NavItem({ to, icon, label, end = false }: { to: string; icon: React.ReactNode; label: string; end?: boolean }) {
  const location = useLocation();
  const currentParams = new URLSearchParams(location.search);
  const [, targetSearch = ""] = to.split("?", 2);
  const targetParams = new URLSearchParams(targetSearch);
  const targetNext = targetParams.get("next");
  const queryAwareActive = targetNext
    ? location.pathname === "/auth" && currentParams.get("next") === targetNext
    : null;

  return <NavLink to={to} end={end} className={({ isActive }) => `nav-item ${(queryAwareActive ?? isActive) ? "active" : ""}`}>{icon}<span>{label}</span></NavLink>;
}

function HomeRoute() {
  const [searchParams] = useSearchParams();
  const sort = searchParams.get("sort");
  if (sort !== "popular" && sort !== "latest") return <HomePage />;

  const next = new URLSearchParams(searchParams);
  next.delete("sort");
  const query = next.toString();
  return <Navigate to={`/${sort}${query ? `?${query}` : ""}`} replace />;
}

export default App;
