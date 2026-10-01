import { Brand, ShellContent, ThemeButton } from "./ShellChrome";
import type { ShellProps } from "./shellTypes";

export function AuthShell({ theme, onThemeChange }: ShellProps) {
  return <div className="auth-shell gv-auth-shell"><a className="gv-skip-link" href="#main-content">跳到主要内容</a><header className="auth-topbar"><Brand /><ThemeButton theme={theme} onChange={onThemeChange} /></header><ShellContent auth /></div>;
}
