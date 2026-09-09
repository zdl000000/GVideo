import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { ImagePlus, X } from "lucide-react";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { Avatar } from "../../shared/components/Avatar";
import { useDialogFocus } from "../../shared/hooks/useDialogFocus";
import type { CreatorProfile, User } from "../../types";

export function EditProfileDialog({ profile, onClose, onSaved }: { profile: CreatorProfile; onClose: () => void; onSaved: (user: User) => void }) {
  const avatarRef = useRef<HTMLInputElement>(null);
  const [username, setUsername] = useState(profile.username);
  const [bio, setBio] = useState(profile.bio);
  const [avatar, setAvatar] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const panelRef = useRef<HTMLElement>(null);
  const avatarPreview = useMemo(() => avatar ? URL.createObjectURL(avatar) : profile.avatar_url, [avatar, profile.avatar_url]);

  useEffect(() => () => { if (avatar) URL.revokeObjectURL(avatarPreview); }, [avatar, avatarPreview]);
  useDialogFocus(panelRef, busy, onClose);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    const form = new FormData();
    form.set("username", username);
    form.set("bio", bio);
    if (avatar) form.set("avatar", avatar);
    try {
      onSaved(await api.updateProfile(form));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
      <section ref={panelRef} className="dialog-panel profile-dialog" role="dialog" aria-modal="true" aria-labelledby="profile-dialog-title">
        <header>
          <div><p className="eyebrow">个人资料</p><h2 id="profile-dialog-title">编辑作者信息</h2></div>
          <button type="button" className="icon-button" onClick={onClose} disabled={busy} aria-label="关闭资料编辑窗口" title="关闭"><X size={19} /></button>
        </header>
        <form className="profile-form" onSubmit={submit}>
          <div className="profile-avatar-editor">
            <Avatar username={username || profile.username} src={avatarPreview} size="large" />
            <button type="button" className="secondary-button compact" onClick={() => avatarRef.current?.click()}><ImagePlus size={16} />更换头像</button>
            <span>JPEG、PNG 或 WebP</span>
            <input ref={avatarRef} className="sr-only" type="file" accept="image/jpeg,image/png,image/webp" onChange={(event) => setAvatar(event.target.files?.[0] || null)} />
          </div>
          <div className="stack-form profile-fields">
            <label>用户名<input autoFocus value={username} onChange={(event) => setUsername(event.target.value)} minLength={3} maxLength={24} required /></label>
            <label>个人简介<textarea value={bio} onChange={(event) => setBio(event.target.value)} maxLength={300} rows={6} placeholder="介绍你的内容方向和创作经历" /><span className="field-count">{bio.length}/300</span></label>
            {error && <p className="inline-error">{error}</p>}
            <div className="dialog-actions"><button type="button" className="secondary-button" onClick={onClose} disabled={busy}>取消</button><button className="primary-button" disabled={busy}>{busy ? "保存中..." : "保存资料"}</button></div>
          </div>
        </form>
      </section>
    </div>
  );
}
