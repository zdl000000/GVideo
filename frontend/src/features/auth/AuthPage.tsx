import { FormEvent, useEffect, useRef, useState } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { Button } from "../../shared/ui/Button";
import mark from "../../assets/gvideo-mark.svg";
import type { AuthPayload } from "../../types";

type AuthMode = "login" | "register";

function authTarget(next: string | null) {
  return next && next.startsWith("/") && !next.startsWith("//") ? next : "/";
}

// 已登录访问 /auth 时按 next 回跳。仅接受站内相对路径（拒绝 // 与绝对
// URL），防开放重定向；注册/登录成功后的跳转与该回跳指向一致，竞态无害。
export function AuthRedirect() {
  const [params] = useSearchParams();
  return <Navigate to={authTarget(params.get("next"))} replace />;
}

export function AuthPage({ onAuth }: { onAuth: (payload: AuthPayload) => void }) {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [mode, setMode] = useState<AuthMode>("login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const submittingControlRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const form = formRef.current, control = submittingControlRef.current;
    if (!form || !control) return;
    if (busy) {
      if (document.activeElement === document.body || form.contains(document.activeElement)) form.focus();
    } else {
      if (document.activeElement === form) control.focus();
      submittingControlRef.current = null;
    }
  }, [busy]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (busy) return;
    submittingControlRef.current = formRef.current?.contains(document.activeElement) ? document.activeElement as HTMLElement : null;
    setBusy(true); setError("");
    try {
      const payload = mode === "login" ? await api.login(username, password) : await api.register(username, password);
      onAuth(payload);
      navigate(authTarget(params.get("next")));
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(false); }
  };

  return (
    <div className="page auth-page gv-auth-experience">
      <section className="auth-intro gv-auth-narrative" aria-labelledby="auth-brand-title">
        <p className="gv-auth-eyebrow">GVIDEO / WATCH · CREATE · SHARE</p>
        <h1 id="auth-brand-title"><span>让内容连接</span><span>更大的世界<span className="gv-auth-title-stop">。</span></span></h1>
        <p className="gv-auth-brand-copy">发现好作品，分享你的视角。</p>
        <div className="auth-steps gv-auth-principles">
          <span><strong>WATCH</strong>发现创作</span><span><strong>CREATE</strong>分享作品</span><span><strong>SHARE</strong>参与讨论</span>
        </div>
        <div className="gv-auth-media-motif" aria-hidden="true"><span /><span /><span /><img src={mark} alt="" /></div>
        <p className="gv-auth-signature">A BRIGHTER WORLD THROUGH VIDEO</p>
      </section>
      <section className="auth-form-wrap gv-auth-form-panel" aria-labelledby="auth-form-title">
        <header className="gv-auth-form-heading"><p className="gv-auth-eyebrow">{mode === "login" ? "LOGIN / 欢迎回来" : "REGISTER / 加入片场"}</p><h2 id="auth-form-title">{mode === "login" ? "登录 GVideo" : "创建 GVideo 账号"}</h2></header>
        <div className="gv-auth-mode" role="group" aria-label="登录或注册">
          <Button variant="ghost" className={mode === "login" ? "active" : ""} aria-pressed={mode === "login"} disabled={busy} onClick={() => setMode("login")}>登录</Button>
          <Button variant="ghost" className={mode === "register" ? "active" : ""} aria-pressed={mode === "register"} disabled={busy} onClick={() => setMode("register")}>注册</Button>
        </div>
        <form ref={formRef} tabIndex={-1} className="stack-form gv-auth-form" onSubmit={submit} aria-busy={busy} aria-labelledby="auth-form-title">
          <label htmlFor="auth-username">用户名<input id="auth-username" name="username" value={username} onChange={(event) => setUsername(event.target.value)} minLength={3} maxLength={24} autoComplete="username" autoCapitalize="none" spellCheck={false} placeholder="3-24 位中文、字母或数字" disabled={busy} required /></label>
          <label htmlFor="auth-password">密码<input id="auth-password" name="password" value={password} onChange={(event) => setPassword(event.target.value)} type="password" minLength={8} maxLength={72} autoComplete={mode === "login" ? "current-password" : "new-password"} placeholder="至少 8 位" disabled={busy} required /></label>
          {error && <p className="inline-error gv-auth-error" role="alert">{error}</p>}
          <Button type="submit" className="gv-auth-submit" disabled={busy}>{busy ? "处理中..." : mode === "login" ? "登录" : "创建账号"}</Button>
          <p className="gv-auth-form-note" role="status">{busy ? (mode === "login" ? "正在登录，请稍候。" : "正在创建账号，请稍候。") : mode === "login" ? "用你的账号，继续发现与创作。" : "从一个账号开始，让你的作品被看见。"}</p>
        </form>
      </section>
    </div>
  );
}
