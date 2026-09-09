import { FormEvent, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import type { AuthPayload } from "../../types";

type AuthMode = "login" | "register";

export function AuthPage({ onAuth }: { onAuth: (payload: AuthPayload) => void }) {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [mode, setMode] = useState<AuthMode>("login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      const payload = mode === "login" ? await api.login(username, password) : await api.register(username, password);
      onAuth(payload);
      navigate(params.get("next") || "/");
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(false); }
  };

  return (
    <div className="page auth-page">
      <section className="auth-intro"><p className="eyebrow">加入片场</p><h1>每一次上传，都是一段时间被认真留下。</h1><div className="auth-steps"><span><strong>01</strong>发现创作</span><span><strong>02</strong>分享作品</span><span><strong>03</strong>参与讨论</span></div></section>
      <section className="auth-form-wrap">
        <div className="segmented-control full"><button className={mode === "login" ? "active" : ""} onClick={() => setMode("login")}>登录</button><button className={mode === "register" ? "active" : ""} onClick={() => setMode("register")}>注册</button></div>
        <form className="stack-form" onSubmit={submit}>
          <label>用户名<input value={username} onChange={(event) => setUsername(event.target.value)} minLength={3} maxLength={24} autoComplete="username" placeholder="3-24 位中文、字母或数字" required /></label>
          <label>密码<input value={password} onChange={(event) => setPassword(event.target.value)} type="password" minLength={8} maxLength={72} autoComplete={mode === "login" ? "current-password" : "new-password"} placeholder="至少 8 位" required /></label>
          {error && <p className="inline-error">{error}</p>}
          <button className="primary-button wide" disabled={busy}>{busy ? "处理中..." : mode === "login" ? "登录" : "创建账号"}</button>
        </form>
      </section>
    </div>
  );
}
