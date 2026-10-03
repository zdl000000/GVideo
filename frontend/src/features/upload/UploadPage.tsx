import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { ArrowUpRight, Check, FileVideo, ImagePlus, MessageCircle, Upload } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { visibilityHelp, visibilityLabels } from "../../shared/lib/visibility";
import { formatFileSize } from "../../shared/lib/format";
import { VideoCover } from "../../shared/components/VideoCover";
import { Button } from "../../shared/ui/Button";
import type { Video } from "../../types";

export function UploadPage() {
  const navigate = useNavigate();
  const videoRef = useRef<HTMLInputElement>(null);
  const coverRef = useRef<HTMLInputElement>(null);
  const subtitleRef = useRef<HTMLInputElement>(null);
  const submitControlsRef = useRef<HTMLDivElement>(null);
  const uploadControlsRef = useRef<HTMLDivElement>(null);
  const uploadFocusRef = useRef<Element | null>(null);
  const [categories, setCategories] = useState<string[]>([]);
  const [videoFile, setVideoFile] = useState<File | null>(null);
  const [coverFile, setCoverFile] = useState<File | null>(null);
  const [subtitleFile, setSubtitleFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [visibility, setVisibility] = useState<Video["visibility"]>("public");
  const [busy, setBusy] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const uploadControllerRef = useRef<AbortController | null>(null);
  const [error, setError] = useState("");

  useEffect(() => { api.categories().then((items) => { setCategories(items); setCategory(items[0] || ""); }).catch((err) => setError(errorMessage(err))); }, []);
  const coverPreview = useMemo(() => coverFile ? URL.createObjectURL(coverFile) : "", [coverFile]);
  useEffect(() => () => { if (coverPreview) URL.revokeObjectURL(coverPreview); }, [coverPreview]);
  useEffect(() => {
    if (!busy) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [busy]);
  useEffect(() => () => uploadControllerRef.current?.abort(), []);
  useEffect(() => {
    if (!uploadFocusRef.current) return;
    const active = document.activeElement;
    const submitButton = submitControlsRef.current?.querySelector("button");
    const cancelButton = uploadControlsRef.current?.querySelector("button");
    if (busy) {
      if (active === document.body || (active === uploadFocusRef.current && active?.matches(":disabled"))) cancelButton?.focus();
    } else {
      if (active === document.body || active === cancelButton) submitButton?.focus();
      uploadFocusRef.current = null;
    }
  }, [busy]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!videoFile) { setError("请选择视频文件"); return; }
    uploadFocusRef.current = event.currentTarget.contains(document.activeElement) ? document.activeElement : null;
    setBusy(true); setUploadProgress(0); setError("");
    const controller = new AbortController();
    uploadControllerRef.current = controller;
    const form = new FormData();
    form.set("title", title); form.set("description", description); form.set("category", category); form.set("visibility", visibility); form.set("video", videoFile);
    if (coverFile) form.set("cover", coverFile);
    if (subtitleFile) { form.set("subtitle", subtitleFile); form.set("subtitle_language", "zh-CN"); form.set("subtitle_label", "中文"); }
    try {
      const created = await api.upload(form, (loaded, total) => {
        setUploadProgress(total > 0 ? Math.round((loaded / total) * 100) : null);
      }, controller.signal);
      uploadFocusRef.current = null;
      navigate(`/video/${created.id}`);
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") setError("上传已取消，文件和表单内容已保留");
      else setError(errorMessage(err));
    } finally {
      uploadControllerRef.current = null;
      setBusy(false);
    }
  };

  const cancelUpload = () => uploadControllerRef.current?.abort();

  return (
    <div className="page gvideo-publish-page">
      <header className="gv-publish-heading"><p className="gv-publish-eyebrow">STUDIO / PUBLISH</p><h1>发布新作品<span aria-hidden="true">↗</span></h1><p>从一段影像开始，让你的创作被看见。</p><span className="gv-publish-heading-note">CREATE / SHARE</span></header>
      <form className="gv-publish-flow" onSubmit={submit} aria-label="发布新作品" aria-busy={busy}>
        <section className="gv-publish-section" aria-labelledby="publish-media-heading">
          <header><span className="gv-publish-number">01</span><p>MEDIA</p><h2 id="publish-media-heading">视频文件</h2></header>
          <div className="gv-publish-section-body">
            <button type="button" className={`gv-publish-file ${videoFile ? "is-selected" : ""}`} disabled={busy} onClick={() => videoRef.current?.click()}>
              <span className="gv-publish-file-icon">{videoFile ? <FileVideo size={28} /> : <Upload size={28} />}</span>
              <span className="gv-publish-file-copy"><strong>{videoFile ? videoFile.name : "选择视频文件"}</strong><span>{videoFile ? `${formatFileSize(videoFile.size)} · ${videoFile.type || "视频文件"}` : "MP4 / WebM / Ogg · 单个文件最大 512 MB"}</span></span>
              <span className="gv-publish-file-action">{videoFile ? "更换文件" : "选择文件"}<ArrowUpRight size={15} /></span>
            </button>
            <input ref={videoRef} className="sr-only" aria-label="视频文件" tabIndex={-1} disabled={busy} type="file" accept="video/mp4,video/webm,video/ogg" onChange={(event) => setVideoFile(event.target.files?.[0] || null)} />
          </div>
        </section>
        <section className="gv-publish-section" aria-labelledby="publish-story-heading">
          <header><span className="gv-publish-number">02</span><p>STORY</p><h2 id="publish-story-heading">作品信息</h2></header>
          <div className="gv-publish-section-body gv-publish-fields">
            <label htmlFor="publish-title">标题<span className="gv-publish-required">必填</span></label>
            <input id="publish-title" value={title} disabled={busy} onChange={(event) => setTitle(event.target.value)} minLength={2} maxLength={80} placeholder="准确说明视频内容" required aria-describedby="publish-title-count" />
            <span id="publish-title-count" className="gv-publish-count">{title.length}/80</span>
            <label htmlFor="publish-description">简介</label>
            <textarea id="publish-description" value={description} disabled={busy} onChange={(event) => setDescription(event.target.value)} maxLength={2000} rows={4} placeholder="补充创作背景、内容提要或相关链接" aria-describedby="publish-description-count" />
            <span id="publish-description-count" className="gv-publish-count">{description.length}/2000</span>
            <label htmlFor="publish-category">分区</label><select id="publish-category" value={category} disabled={busy} onChange={(event) => setCategory(event.target.value)} required>{categories.map((item) => <option key={item}>{item}</option>)}</select>
          </div>
        </section>
        <section className="gv-publish-section" aria-labelledby="publish-cover-heading">
          <header><span className="gv-publish-number">03</span><p>COVER</p><h2 id="publish-cover-heading">视频封面</h2></header>
          <div className="gv-publish-section-body gv-publish-cover-layout">
            <button type="button" className="gv-publish-cover" aria-label={coverFile ? "更换自定义封面" : "选择自定义封面"} disabled={busy} onClick={() => coverRef.current?.click()}>
              {coverPreview ? <img src={coverPreview} alt="封面预览" /> : <VideoCover src="" />}
              <span><ImagePlus size={15} />{coverFile ? "更换封面" : "选择自定义封面"}</span>
            </button>
            <div className="gv-publish-cover-copy"><p>{coverFile ? coverFile.name : "为作品选择一张真实封面"}</p><span>{coverFile ? formatFileSize(coverFile.size) : "JPEG / PNG / WebP"}</span><small>未选择封面时会自动截取视频画面</small></div>
            <input ref={coverRef} className="sr-only" aria-label="封面图片" tabIndex={-1} disabled={busy} type="file" accept="image/jpeg,image/png,image/webp" onChange={(event) => setCoverFile(event.target.files?.[0] || null)} />
          </div>
        </section>
        <section className="gv-publish-section" aria-labelledby="publish-subtitle-heading">
          <header><span className="gv-publish-number">04</span><p>SUBTITLE</p><h2 id="publish-subtitle-heading">初始中文字幕</h2></header>
          <div className="gv-publish-section-body">
            <button type="button" className="gv-publish-subtitle" disabled={busy} onClick={() => subtitleRef.current?.click()}><MessageCircle size={22} /><span><strong>{subtitleFile ? subtitleFile.name : "添加字幕文件（可选）"}</strong><small>{subtitleFile ? `${formatFileSize(subtitleFile.size)} · 中文 / zh-CN` : "单条初始中文字幕 · VTT / SRT，最大 2 MB"}</small></span><span>{subtitleFile ? "更换" : "选择"}<ArrowUpRight size={14} /></span></button>
            <input ref={subtitleRef} className="sr-only" aria-label="字幕文件" tabIndex={-1} disabled={busy} type="file" accept=".vtt,.srt,text/vtt,application/x-subrip" onChange={(event) => setSubtitleFile(event.target.files?.[0] || null)} />
            <p className="gv-publish-help">发布后可在投稿管理的字幕弹窗中管理更多轨道。</p>
          </div>
        </section>
        <section className="gv-publish-section" aria-labelledby="publish-visibility-heading">
          <header><span className="gv-publish-number">05</span><p>VISIBILITY</p><h2 id="publish-visibility-heading">可见范围</h2></header>
          <fieldset className="gv-publish-section-body gv-publish-visibility" disabled={busy}><legend className="sr-only">可见范围</legend>{Object.entries(visibilityLabels).map(([value, label]) => <label key={value} className={visibility === value ? "is-selected" : ""}><input type="radio" name="visibility" value={value} checked={visibility === value} onChange={() => setVisibility(value as Video["visibility"])} /><span><strong>{label}</strong><small>{visibilityHelp[value as Video["visibility"]]}</small></span>{visibility === value && <Check size={15} aria-hidden="true" />}</label>)}</fieldset>
        </section>
        <section className="gv-publish-section gv-publish-final" aria-labelledby="publish-submit-heading">
          <header><span className="gv-publish-number">06</span><p>PUBLISH</p><h2 id="publish-submit-heading">准备发布</h2></header>
          <div className="gv-publish-section-body">
            {error && <p className="inline-error" role="alert">{error}</p>}
            {busy && <div ref={uploadControlsRef} className="gv-publish-progress" role="status" aria-live="polite">
              <div className="gv-publish-progress-copy"><span>正在上传文件</span><strong>{uploadProgress === null ? "计算中" : `${uploadProgress}%`}</strong></div>
              <div className="gv-publish-progress-track" role="progressbar" aria-label="文件上传进度" aria-valuemin={0} aria-valuemax={100} aria-valuenow={uploadProgress ?? undefined} aria-valuetext={uploadProgress === null ? "上传大小未知" : `${uploadProgress}%`}><span style={{ width: `${uploadProgress ?? 0}%` }} /></div>
              <p>上传完成后会进入后台处理阶段</p>
              <Button variant="secondary" onClick={cancelUpload}>取消上传</Button>
            </div>}
            <div ref={submitControlsRef} className="gv-publish-submit"><p>文件与作品信息将一并提交。<br /><span>上传期间请保持此页面打开。</span></p><Button type="submit" disabled={busy || !videoFile}>{busy ? "正在上传..." : "发布作品"}<ArrowUpRight size={17} /></Button></div>
          </div>
        </section>
      </form>
    </div>
  );
}
