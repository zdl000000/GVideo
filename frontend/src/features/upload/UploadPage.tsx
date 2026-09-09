import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { Film, MessageCircle, Upload } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { visibilityHelp, visibilityLabels } from "../../shared/lib/visibility";
import type { Video } from "../../types";

export function UploadPage() {
  const navigate = useNavigate();
  const videoRef = useRef<HTMLInputElement>(null);
  const coverRef = useRef<HTMLInputElement>(null);
  const subtitleRef = useRef<HTMLInputElement>(null);
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

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!videoFile) { setError("请选择视频文件"); return; }
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
    <div className="page upload-page">
      <section className="page-heading"><p className="eyebrow">创作中心</p><h1>发布新作品</h1><p>支持 MP4、WebM、Ogg，未选择封面时会自动截取视频画面。</p></section>
      <form className="upload-layout" onSubmit={submit}>
        <div className="upload-media-column">
          <button type="button" className={`file-drop ${videoFile ? "selected" : ""}`} onClick={() => videoRef.current?.click()}>
            <Upload size={30} /><strong>{videoFile ? videoFile.name : "选择视频文件"}</strong><span>{videoFile ? `${(videoFile.size / 1024 / 1024).toFixed(1)} MB` : "单个文件最大 512 MB"}</span>
          </button>
          <input ref={videoRef} className="sr-only" type="file" accept="video/mp4,video/webm,video/ogg" onChange={(event) => setVideoFile(event.target.files?.[0] || null)} />
          <button type="button" className="cover-picker" onClick={() => coverRef.current?.click()}>
            {coverPreview ? <img src={coverPreview} alt="封面预览" /> : <><Film size={24} /><span>选择自定义封面</span></>}
          </button>
          <input ref={coverRef} className="sr-only" type="file" accept="image/jpeg,image/png,image/webp" onChange={(event) => setCoverFile(event.target.files?.[0] || null)} />
          <button type="button" className={`subtitle-picker ${subtitleFile ? "selected" : ""}`} onClick={() => subtitleRef.current?.click()}>
            <MessageCircle size={20} /><span>{subtitleFile ? subtitleFile.name : "添加字幕文件（可选）"}</span><small>支持 VTT / SRT，最大 2 MB</small>
          </button>
          <input ref={subtitleRef} className="sr-only" type="file" accept=".vtt,.srt,text/vtt,application/x-subrip" onChange={(event) => setSubtitleFile(event.target.files?.[0] || null)} />
        </div>
        <div className="stack-form upload-fields">
          <label>标题<input value={title} onChange={(event) => setTitle(event.target.value)} minLength={2} maxLength={80} placeholder="准确说明视频内容" required /><span className="field-count">{title.length}/80</span></label>
          <label>分区<select value={category} onChange={(event) => setCategory(event.target.value)} required>{categories.map((item) => <option key={item}>{item}</option>)}</select></label>
          <label>可见范围<select value={visibility} onChange={(event) => setVisibility(event.target.value as Video["visibility"])}>{Object.entries(visibilityLabels).map(([value, label]) => <option value={value} key={value}>{label}</option>)}</select><span className="field-help">{visibilityHelp[visibility]}</span></label>
          <label>简介<textarea value={description} onChange={(event) => setDescription(event.target.value)} maxLength={2000} rows={8} placeholder="补充创作背景、内容提要或相关链接" /><span className="field-count">{description.length}/2000</span></label>
          {error && <p className="inline-error">{error}</p>}
          {busy && <div className="upload-progress" role="status">
            <div className="upload-progress-copy"><span>正在上传文件</span><strong>{uploadProgress === null ? "计算中" : `${uploadProgress}%`}</strong></div>
            <div className="upload-progress-track"><span style={{ width: `${uploadProgress ?? 0}%` }} /></div>
            <small>上传完成后会进入后台处理阶段</small>
            <button type="button" className="secondary-button" onClick={cancelUpload}>取消上传</button>
          </div>}
          <button className="primary-button wide" disabled={busy || !videoFile}>{busy ? "正在上传..." : "发布作品"}</button>
        </div>
      </form>
    </div>
  );
}
