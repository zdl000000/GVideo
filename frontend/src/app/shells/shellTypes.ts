import type { RefObject } from "react";
import type { User } from "../../types";

export type Theme = "light" | "dark";

export interface AuthState {
  user: User | null;
  loading: boolean;
}

export interface ShellProps {
  auth: AuthState;
  theme: Theme;
  onThemeChange: () => void;
  onLogout: () => void;
}

export interface ShellNavigation {
  query: string;
  setQuery: (query: string) => void;
  search: () => void;
  logout: () => Promise<void>;
  unreadNotifications: number;
  menuOpen: boolean;
  setMenuOpen: (open: boolean) => void;
  isMobile: boolean;
  menuButtonRef: RefObject<HTMLButtonElement | null>;
  drawerRef: RefObject<HTMLElement | null>;
}

export type WorkspaceShellProps = ShellProps & { navigation: ShellNavigation };
