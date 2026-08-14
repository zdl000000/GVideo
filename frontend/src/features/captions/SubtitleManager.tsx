import { FormEvent, useRef, useState } from "react";
import { Check, CircleCheck, Trash2, Upload } from "lucide-react";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import type { SubtitleTrack, Video } from "../../types";

interface SubtitleManagerProps {
  video: Video;
  onTracksChanged: (tracks: SubtitleTrack[]) => void;
}

export function SubtitleManager({ video, onTracksChanged }: SubtitleManagerProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [language, setLanguage] = useState("zh-CN");
  const [label, setLabel] = useState("中文");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const tracks = video.subtitle_tracks ?? [];

  const showNotice = (message: string) => {
    setNotice(message);
    window.setTimeout(() => setNotice(""), 2500);
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!file) return;
    setBusy("upload"); setError(""); setNotice("");
    const form = new FormData();
    form.set("subtitle", file);
    form.set("subtitle_language", language.trim() || "zh-CN");
    form.set("subtitle_label", label.trim() || language.trim() || "字幕");
    try {
      const created = await api.uploadSubtitle(video.id, form);
      const nextTracks = created.is_default
        ? [...tracks.map((track) => ({ ...track, is_default: false })), created]
        : [...tracks, created];
      onTracksChanged(nextTracks);
      setFile(null);
      if (inputRef.current) inputRef.current.value = "";
      showNotice("字幕轨道已上传");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy("");
    }
  };

  const setDefault = async (track: SubtitleTrack) => {
    setBusy(`default-${track.id}`); setError(""); setNotice("");
    try {
      onTracksChanged(await api.setDefaultSubtitle(video.id, track.id));
      showNotice(`已将“${track.label}”设为默认字幕`);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy("");
    }
  };

  const remove = async (track: SubtitleTrack) => {
    if (!window.confirm(`确定删除字幕轨道“${track.label}”吗？`)) return;
    setBusy(`delete-${track.id}`); setError(""); setNotice("");
    try {
      onTracksChanged(await api.deleteSubtitle(video.id, track.id));
      showNotice(`已删除“${track.label}”字幕`);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy("");
    }
  };

  return (
    <section className="subtitle-manager" aria-labelledby={`subtitle-manager-${video.id}`} aria-busy={Boolean(busy)}>
      <header>
        <div><strong id={`subtitle-manager-${video.id}`}>字幕管理</strong><span>管理语言轨道与默认字幕，支持 VTT 或 SRT，最大 2 MB</span></div>
        <span>{tracks.length} 条轨道</span>
      </header>
      {tracks.length ? (
        <div className="subtitle-track-list">
          {tracks.map((track) => (
            <article className="subtitle-track-row" key={track.id}>
              <div className="subtitle-track-copy">
                <strong>{track.label}</strong>
                <span><code>{track.language}</code>{track.is_default && <b><CircleCheck size={13} />默认</b>}</span>
              </div>
              <div className="subtitle-track-actions">
                {!track.is_default && <button type="button" disabled={Boolean(busy)} onClick={() => setDefault(track)} title="设为默认字幕"><Check size={15} />{busy === `default-${track.id}` ? "设置中..." : "设为默认"}</button>}
                <button type="button" className="danger" disabled={Boolean(busy)} onClick={() => remove(track)} title="删除字幕轨道"><Trash2 size={15} />{busy === `delete-${track.id}` ? "删除中..." : "删除"}</button>
              </div>
            </article>
          ))}
        </div>
      ) : <p className="subtitle-track-empty">暂无字幕轨道，可以在下方上传第一条字幕。</p>}
      <form className="subtitle-upload-form" onSubmit={submit}>
        <label>语言代码<input value={language} onChange={(event) => setLanguage(event.target.value)} maxLength={24} placeholder="zh-CN" disabled={Boolean(busy)} required /></label>
        <label>显示标签<input value={label} onChange={(event) => setLabel(event.target.value)} maxLength={40} placeholder="中文" disabled={Boolean(busy)} required /></label>
        <label className="subtitle-file-field">字幕文件<input ref={inputRef} type="file" accept=".vtt,.srt,text/vtt,application/x-subrip" onChange={(event) => setFile(event.target.files?.[0] || null)} disabled={Boolean(busy)} required /></label>
        <button className="secondary-button compact" disabled={!file || Boolean(busy)}><Upload size={15} />{busy === "upload" ? "上传中..." : "上传轨道"}</button>
      </form>
      {notice && <p className="subtitle-notice" role="status"><CircleCheck size={15} />{notice}</p>}
      {error && <p className="inline-error">{error}</p>}
    </section>
  );
}
