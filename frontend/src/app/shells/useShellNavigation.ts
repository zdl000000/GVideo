import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ShellNavigation, ShellProps } from "./shellTypes";

export function useShellNavigation({ auth, onLogout }: ShellProps): ShellNavigation {
  const navigate = useNavigate();
  const location = useLocation();
  const [query, setQuery] = useState("");
  const [menuOpen, setMenuOpen] = useState(false);
  const [unreadNotifications, setUnreadNotifications] = useState(0);
  const [isMobile, setIsMobile] = useState(() => window.matchMedia("(max-width: 991px)").matches);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const drawerRef = useRef<HTMLElement>(null);
  const wasMenuOpen = useRef(false);

  useEffect(() => setMenuOpen(false), [location.pathname, location.search]);

  useEffect(() => {
    if (!auth.user) {
      setUnreadNotifications(0);
      return;
    }
    const controller = new AbortController();
    const refresh = () => {
      const params = new URLSearchParams({ page_size: "1" });
      api.notifications(params, controller.signal)
        .then((result) => { if (!controller.signal.aborted) setUnreadNotifications(result.unread_count); })
        .catch(() => undefined);
    };
    refresh();
    window.addEventListener("gvideo-notifications-changed", refresh);
    return () => {
      controller.abort();
      window.removeEventListener("gvideo-notifications-changed", refresh);
    };
  }, [auth.user, location.pathname]);

  useEffect(() => {
    const mediaQuery = window.matchMedia("(max-width: 991px)");
    const update = () => {
      setIsMobile(mediaQuery.matches);
      if (!mediaQuery.matches) setMenuOpen(false);
    };
    update();
    mediaQuery.addEventListener("change", update);
    return () => mediaQuery.removeEventListener("change", update);
  }, []);

  useLayoutEffect(() => {
    if (!menuOpen || !isMobile || !drawerRef.current) {
      if (wasMenuOpen.current) {
        // Resizing to desktop hides the drawer trigger; keep focus in visible content.
        (isMobile ? menuButtonRef.current ?? document.getElementById("main-content") : document.getElementById("main-content"))?.focus();
      }
      wasMenuOpen.current = false;
      return;
    }
    wasMenuOpen.current = true;
    const panel = drawerRef.current;
    // Only the drawer stays interactive while open; restore pre-existing state.
    const background = Array.from(panel.parentElement?.children ?? [])
      .filter((element) => element !== panel && !element.classList.contains("gv-drawer-scrim")) as HTMLElement[];
    const previous = background.map((element) => ({ element, inert: element.inert }));
    const previousOverflow = document.body.style.overflow;
    background.forEach((element) => { element.inert = true; });
    document.body.style.overflow = "hidden";
    const focusable = () => Array.from(panel.querySelectorAll<HTMLElement>("a[href],button:not(:disabled),input:not(:disabled),[tabindex]:not([tabindex='-1'])"));
    focusable()[0]?.focus();
    // CSS visibility transitions can delay focusability until the next frame.
    const focusFrame = window.requestAnimationFrame(() => focusable()[0]?.focus());
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setMenuOpen(false);
      }
      if (event.key !== "Tab") return;
      const elements = focusable();
      const first = elements[0];
      const last = elements[elements.length - 1];
      if (!first || !last) return;
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      window.cancelAnimationFrame(focusFrame);
      previous.forEach(({ element, inert }) => { element.inert = inert; });
      document.body.style.overflow = previousOverflow;
    };
  }, [menuOpen, isMobile]);

  const logout = async () => {
    await api.logout().catch(console.error);
    setMenuOpen(false);
    onLogout();
    navigate("/");
  };

  return {
    query, setQuery, menuOpen, setMenuOpen, isMobile,
    unreadNotifications, menuButtonRef, drawerRef, logout,
    search: () => navigate(query.trim() ? `/?q=${encodeURIComponent(query.trim())}` : "/")
  };
}
