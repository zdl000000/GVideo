import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import {
  CircleCheck,
  Eye,
  Film,
  Heart,
  History,
  ImagePlus,
  MessageCircle,
  Pencil,
  Play,
  RefreshCw,
  Trash2,
  Upload,
  X
} from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { EmptyState } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { ProcessingBadge, ProcessingProgress } from "../../shared/components/Processing";
import { useDialogFocus } from "../../shared/hooks/useDialogFocus";
import { errorMessage } from "../../shared/lib/errors";
import { formatCount, formatDate, formatDuration, formatFileSize, pageFrom } from "../../shared/lib/format";
import { visibilityHelp, visibilityLabels } from "../../shared/lib/visibility";
import { SubtitleManager } from "../captions/SubtitleManager";
import type { SubtitleTrack, Video, VideoPage as VideoPageData } from "../../types";

export function ManagementLoading() { return <div className="management-list">{Array.from({ length: 5 }, (_, index) => <div className="management-skeleton" key={index}><span /><div><i /><i /><i /></div></div>)}</div>; }

export function EditVideoDialog({ video, categories, onClose, onSaved }: { video: Video; categories: string[]; onClose: () => void; onSaved: (video: Video) => void }) {
  const coverRef = useRef<HTMLInputElement>(null);
  const [title, setTitle] = useState(video.title);
  const [description, setDescription] = useState(video.description);
  const [category, setCategory] = useState(video.category);
  const [visibility, setVisibility] = useState<Video["visibility"]>(video.visibility);
  const [cover, setCover] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const panelRef = useRef<HTMLElement>(null);
  const coverPreview = useMemo(() => cover ? URL.createObjectURL(cover) : video.cover_url, [cover, video.cover_url]);
  useEffect(() => () => { if (cover) URL.revokeObjectURL(coverPreview); }, [cover, coverPreview]);
  useDialogFocus(panelRef, busy, onClose);
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true); setError("");
    const form = new FormData();
    form.set("title", title); form.set("description", description); form.set("category", category); form.set("visibility", visibility);
    if (cover) form.set("cover", cover);
    try { onSaved(await api.updateVideo(video.id, form)); }
    catch (err) { setError(errorMessage(err)); } finally { setBusy(false); }
  };
  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
      <section ref={panelRef} className="dialog-panel edit-video-dialog" role="dialog" aria-modal="true" aria-labelledby="edit-video-title">
        <header><div><p className="eyebrow">投稿管理</p><h2 id="edit-video-title">编辑视频信息</h2></div><button type="button" className="icon-button" onClick={onClose} disabled={busy} aria-label="关闭编辑窗口" title="关闭"><X size={19} /></button></header>
        <form className="edit-video-form" onSubmit={submit}>
          <button type="button" className="edit-cover" onClick={() => coverRef.current?.click()}>
            {coverPreview ? <img src={coverPreview} alt="当前封面" /> : <div className="cover-fallback"><Film size={28} /></div>}
            <span><ImagePlus size={16} />{cover ? "已选择新封面" : "替换封面"}</span>
          </button>
          <input ref={coverRef} className="sr-only" type="file" accept="image/jpeg,image/png,image/webp" onChange={(event) => setCover(event.target.files?.[0] || null)} />
          <div className="stack-form edit-video-fields">
            <label>标题<input value={title} onChange={(event) => setTitle(event.target.value)} minLength={2} maxLength={80} required /><span className="field-count">{title.length}/80</span></label>
            <label>分区<select value={category} onChange={(event) => setCategory(event.target.value)} required>{(categories.length ? categories : [video.category]).map((item) => <option key={item}>{item}</option>)}</select></label>
            <label>可见范围<select value={visibility} onChange={(event) => setVisibility(event.target.value as Video["visibility"])}>{Object.entries(visibilityLabels).map(([value, label]) => <option value={value} key={value}>{label}</option>)}</select><span className="field-help">{visibilityHelp[visibility]}</span></label>
            <label>简介<textarea value={description} onChange={(event) => setDescription(event.target.value)} maxLength={2000} rows={6} placeholder="补充视频内容和创作背景" /><span className="field-count">{description.length}/2000</span></label>
            {error && <p className="inline-error">{error}</p>}
            <div className="dialog-actions"><button type="button" className="secondary-button" onClick={onClose} disabled={busy}>取消</button><button className="primary-button" disabled={busy}>{busy ? "保存中..." : "保存修改"}</button></div>
          </div>
        </form>
      </section>
    </div>
  );
}

export function SubtitleDialog({ video, onClose, onTracksChanged }: { video: Video; onClose: () => void; onTracksChanged: (tracks: SubtitleTrack[]) => void }) {
  const panelRef = useRef<HTMLElement>(null);
  useDialogFocus(panelRef, false, onClose);
  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <section ref={panelRef} className="dialog-panel subtitle-dialog" role="dialog" aria-modal="true" aria-labelledby={`subtitle-dialog-title-${video.id}`}>
        <header>
          <div>
            <p className="eyebrow">投稿管理</p>
            <h2 id={`subtitle-dialog-title-${video.id}`}>字幕管理</h2>
            <p className="dialog-context" title={video.title}>{video.title}</p>
          </div>
          <button type="button" className="icon-button" onClick={onClose} aria-label="关闭字幕管理" title="关闭"><X size={19} /></button>
        </header>
        <SubtitleManager video={video} onTracksChanged={onTracksChanged} />
      </section>
    </div>
  );
}

export function DeleteVideoDialog({ video, busy, onClose, onConfirm }: { video: Video; busy: boolean; onClose: () => void; onConfirm: () => void }) {
  const panelRef = useRef<HTMLElement>(null);
  useDialogFocus(panelRef, busy, onClose);
  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
      <section ref={panelRef} className="dialog-panel delete-video-dialog" role="alertdialog" aria-modal="true" aria-labelledby="delete-video-title" aria-describedby="delete-video-description">
        <span className="danger-icon"><Trash2 size={22} /></span>
        <h2 id="delete-video-title">删除这条投稿？</h2>
        <p id="delete-video-description">“{video.title}”的数据库记录、原视频、封面、字幕和转码文件都会被永久删除，此操作无法撤销。</p>
        <div className="dialog-actions"><button type="button" className="secondary-button" onClick={onClose} disabled={busy}>取消</button><button type="button" className="danger-button" onClick={onConfirm} disabled={busy}>{busy ? "正在删除..." : "确认删除"}</button></div>
      </section>
    </div>
  );
}

export function MyVideosPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 12, total: 0, has_next: false });
  const [categories, setCategories] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [editing, setEditing] = useState<Video | null>(null);
  const [subtitleEditing, setSubtitleEditing] = useState<Video | null>(null);
  const [deleting, setDeleting] = useState<Video | null>(null);
  const [busy, setBusy] = useState("");
  const listRequestRef = useRef<AbortController | null>(null);
  const dialogTriggerRef = useRef<HTMLElement | null>(null);
  const pageHeadingRef = useRef<HTMLHeadingElement>(null);
  const page = pageFrom(searchParams);
  const videos = result.items;

  const requestPage = (pageNumber: number, signal?: AbortSignal) => api.myVideos(new URLSearchParams({ page: String(pageNumber), page_size: "12" }), signal);
  const refreshPage = async (pageNumber: number) => {
    listRequestRef.current?.abort();
    const controller = new AbortController();
    listRequestRef.current = controller;
    try {
      const next = await requestPage(pageNumber, controller.signal);
      if (listRequestRef.current !== controller || controller.signal.aborted) return false;
      setResult(next);
      return true;
    } finally {
      if (listRequestRef.current === controller) listRequestRef.current = null;
    }
  };
  const closeDialog = (setter: (value: null) => void) => {
    setter(null);
    window.setTimeout(() => dialogTriggerRef.current?.focus(), 0);
  };

  useEffect(() => { api.categories().then(setCategories).catch(console.error); }, []);
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    refreshPage(page).catch((err) => { if (active && !(err instanceof DOMException && err.name === "AbortError")) setError(errorMessage(err)); }).finally(() => { if (active) setLoading(false); });
    return () => {
      active = false;
      listRequestRef.current?.abort();
    };
  }, [page]);

  const processingKey = videos.filter((video) => video.processing_status === "pending" || video.processing_status === "processing").map((video) => video.id).join(",");
  useEffect(() => {
    if (!processingKey) return;
    let stopped = false;
    let timer = 0;
    let controller: AbortController | null = null;
    const poll = async () => {
      controller = new AbortController();
      try {
        const next = await requestPage(page, controller.signal);
        if (!stopped) setResult(next);
      } catch (err) {
        if (!stopped && !(err instanceof DOMException && err.name === "AbortError")) console.error(err);
      } finally {
        controller = null;
        if (!stopped) timer = window.setTimeout(poll, 2500);
      }
    };
    timer = window.setTimeout(poll, 2500);
    return () => {
      stopped = true;
      window.clearTimeout(timer);
      controller?.abort();
    };
  }, [page, processingKey]);

  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };
  const showNotice = (message: string) => {
    setNotice(message);
    window.setTimeout(() => setNotice(""), 3000);
  };
  const retry = async (video: Video) => {
    setBusy(`retry-${video.id}`); setError("");
    try {
      await api.retryVideo(video.id);
      setResult((current) => ({ ...current, items: current.items.map((item) => item.id === video.id ? { ...item, processing_status: "pending", processing_error: undefined } : item) }));
      showNotice("已重新加入转码队列");
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(""); }
  };
  const remove = async (video: Video) => {
    setBusy(`delete-${video.id}`); setError("");
    try {
      await api.deleteVideo(video.id);
      setDeleting(null);
      if (videos.length === 1 && page > 1) {
        setPage(page - 1);
      } else {
        await refreshPage(page);
      }
      window.setTimeout(() => pageHeadingRef.current?.focus(), 0);
      showNotice("投稿及其媒体文件已删除");
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(""); }
  };
  const applyUpdated = (updated: Video) => {
    setResult((current) => ({ ...current, items: current.items.map((video) => video.id === updated.id ? updated : video) }));
    setEditing(null);
    showNotice("投稿信息已保存");
  };

  return (
    <div className="page management-page">
      <section className="page-heading heading-row"><div><p className="eyebrow">个人空间</p><h1 ref={pageHeadingRef} tabIndex={-1}>我的投稿</h1><p>查看作品表现，继续完善你的创作列表。</p></div><Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link></section>
      <div className="management-summary"><span><strong>{result.total}</strong> 条投稿</span><span>每页 12 条</span></div>
      {notice && <div className="management-notice" role="status"><CircleCheck size={17} />{notice}</div>}
      {error && <p className="inline-error management-error">{error}</p>}
      {loading ? <ManagementLoading /> : videos.length ? (
        <>
          <section className="management-list" aria-label="投稿列表">
            {videos.map((video) => (
              <article className="management-row" key={video.id}>
                <Link to={`/video/${video.id}`} className="management-cover">
                  {video.cover_url ? <img src={video.cover_url} alt="" loading="lazy" /> : <div className="cover-fallback"><Play size={24} fill="currentColor" /></div>}
                  <span className="duration">{formatDuration(video.duration_seconds)}</span>
                </Link>
                <div className="management-copy">
                  <div className="management-title-line"><Link to={`/video/${video.id}`}>{video.title}</Link><ProcessingBadge video={video} /></div>
                  <p>{video.description || "暂未填写视频简介"}</p>
                  <div className="management-meta"><span>{video.category}</span><span className={`visibility-badge ${video.visibility}`}>{visibilityLabels[video.visibility]}</span><span>{formatDate(video.created_at)}</span><span>{formatFileSize(video.size_bytes)}</span></div>
                  {(video.processing_status === "pending" || video.processing_status === "processing") && <ProcessingProgress video={video} compact />}
                  {video.processing_status === "failed" && <span className="management-processing-error">{video.processing_error || "媒体处理失败，可以手动重新转码"}</span>}
                  <div className="management-stats"><span><Eye size={14} />{formatCount(video.views_count)} 播放</span><span><Heart size={14} />{formatCount(video.likes_count)}</span><span><MessageCircle size={14} />{formatCount(video.comments_count)}</span></div>
                </div>
                <div className="management-actions" aria-label={`${video.title}的管理操作`}>
                  <button type="button" onClick={(event) => { dialogTriggerRef.current = event.currentTarget; setEditing(video); }} title="编辑投稿"><Pencil size={16} />编辑</button>
                  <button type="button" onClick={(event) => { dialogTriggerRef.current = event.currentTarget; setSubtitleEditing(video); }} title="管理字幕"><MessageCircle size={16} />字幕</button>
                  {video.processing_status === "failed" && <button type="button" disabled={busy === `retry-${video.id}`} onClick={() => retry(video)} title="重新转码"><RefreshCw size={16} className={busy === `retry-${video.id}` ? "spin-icon" : ""} />重试</button>}
                  <button type="button" className="danger" disabled={video.processing_status === "processing" || busy === `delete-${video.id}`} onClick={(event) => { dialogTriggerRef.current = event.currentTarget; setDeleting(video); }} title={video.processing_status === "processing" ? "转码完成后才能删除" : "删除投稿"}><Trash2 size={16} />删除</button>
                </div>
              </article>
            ))}
          </section>
          <Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} />
        </>
      ) : <EmptyState icon={<History size={28} />} title="还没有投稿" text="第一条作品不必完美，先让它可以被看见。" action={<Link to="/upload" className="primary-button"><Upload size={17} />开始投稿</Link>} />}
      {editing && <EditVideoDialog video={editing} categories={categories} onClose={() => closeDialog(setEditing)} onSaved={(updated) => { applyUpdated(updated); window.setTimeout(() => dialogTriggerRef.current?.focus(), 0); }} />}
      {subtitleEditing && <SubtitleDialog video={subtitleEditing} onClose={() => closeDialog(setSubtitleEditing)} onTracksChanged={(tracks) => {
        setSubtitleEditing((current) => current ? { ...current, subtitle_tracks: tracks } : current);
        setResult((current) => ({ ...current, items: current.items.map((item) => item.id === subtitleEditing.id ? { ...item, subtitle_tracks: tracks } : item) }));
      }} />}
      {deleting && <DeleteVideoDialog video={deleting} busy={busy === `delete-${deleting.id}`} onClose={() => closeDialog(setDeleting)} onConfirm={() => remove(deleting)} />}
    </div>
  );
}
