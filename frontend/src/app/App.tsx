import { lazy, useEffect, useRef, useState } from "react";
import { Navigate, Route, Routes, useLocation, useSearchParams } from "react-router-dom";
import { ApiError, api, setCSRFToken } from "../shared/api/client";
import { LoadingBlock, NotFound } from "../shared/components/Feedback";
import { ApplicationShell } from "./shells/ApplicationShell";
import type { AuthState, Theme } from "./shells/shellTypes";
import type { AuthPayload, User } from "../types";

const AuthPage = lazy(() => import("../features/auth/AuthPage").then((module) => ({ default: module.AuthPage })));
const AuthRedirect = lazy(() => import("../features/auth/AuthPage").then((module) => ({ default: module.AuthRedirect })));
const CreatorDashboard = lazy(() => import("../features/creator/CreatorDashboard").then((module) => ({ default: module.CreatorDashboard })));
const CreatorPage = lazy(() => import("../features/creator/CreatorPage").then((module) => ({ default: module.CreatorPage })));
const NotificationCenterPage = lazy(() => import("../features/notifications/NotificationCenterPage").then((module) => ({ default: module.NotificationCenterPage })));
const AdminReportsPage = lazy(() => import("../features/moderation/AdminReportsPage").then((module) => ({ default: module.AdminReportsPage })));
const FavoritesPage = lazy(() => import("../features/videos/FavoritesPage").then((module) => ({ default: module.FavoritesPage })));
const FollowingPage = lazy(() => import("../features/videos/FollowingPage").then((module) => ({ default: module.FollowingPage })));
const HomePage = lazy(() => import("../features/videos/HomePage").then((module) => ({ default: module.HomePage })));
const LatestPage = lazy(() => import("../features/videos/LatestPage").then((module) => ({ default: module.LatestPage })));
const MyVideosPage = lazy(() => import("../features/videos/MyVideosPage").then((module) => ({ default: module.MyVideosPage })));
const PopularPage = lazy(() => import("../features/videos/PopularPage").then((module) => ({ default: module.PopularPage })));
const UploadPage = lazy(() => import("../features/upload/UploadPage").then((module) => ({ default: module.UploadPage })));
const VideoPage = lazy(() => import("../features/watch/VideoPage").then((module) => ({ default: module.VideoPage })));

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
    document.querySelector('meta[name="theme-color"]')?.setAttribute("content", theme === "dark" ? "#080a0d" : "#f7f8fa");
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
      <Route element={<ApplicationShell auth={auth} onLogout={clearAuth} theme={theme} onThemeChange={() => setTheme((current) => current === "light" ? "dark" : "light")} />}>
        <Route index element={<HomeRoute />} />
        <Route path="latest" element={<LatestPage />} />
        <Route path="popular" element={<PopularPage />} />
        <Route path="following" element={<Protected auth={auth}><FollowingPage /></Protected>} />
        <Route path="favorites" element={<Protected auth={auth}><FavoritesPage /></Protected>} />
        <Route path="users/:id" element={<CreatorPage user={auth.user} onUserUpdated={updateAuthUser} />} />
        <Route path="video/:id" element={<VideoPage user={auth.user} />} />
        <Route path="auth" element={auth.user ? <AuthRedirect /> : <AuthPage onAuth={applyAuth} />} />
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
