import { FormEvent, useEffect, useState } from "react";
import { Bookmark, Check, Clock3, Eye, Film, Flag, Heart, LayoutDashboard, Play, Send, Share2, Trash2, UserRound, X } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { Avatar } from "../../shared/components/Avatar";
import { ErrorBlock, LoadingBlock } from "../../shared/components/Feedback";
import { ProcessingBadge, ProcessingProgress } from "../../shared/components/Processing";
import { errorMessage } from "../../shared/lib/errors";
import { formatCount, formatDate, formatDuration } from "../../shared/lib/format";
import type { Comment, CreatorProfile, User, Video } from "../../types";
import { VideoPlayer } from "./VideoPlayer";

export function VideoPage({ user }: { user: User | null }) {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [video, setVideo] = useState<Video | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [relatedVideos, setRelatedVideos] = useState<Video[]>([]);
  const [authorProfile, setAuthorProfile] = useState<CreatorProfile | null>(null);
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);
  const [relatedLoading, setRelatedLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [shareStatus, setShareStatus] = useState("");
  const [reportOpen, setReportOpen] = useState(false);
  const [reportReason, setReportReason] = useState("spam");
  const [reportDetail, setReportDetail] = useState("");
  const [reportBusy, setReportBusy] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setRelatedLoading(true);
    setError("");
    setRelatedVideos([]);
    setAuthorProfile(null);
    Promise.all([api.video(id, true, controller.signal), api.comments(id, controller.signal)])
      .then(([nextVideo, nextComments]) => {
        if (controller.signal.aborted) return;
        setVideo(nextVideo);
        setComments(nextComments);
        api.creator(String(nextVideo.user_id), controller.signal)
          .then((profile) => { if (!controller.signal.aborted) setAuthorProfile(profile); })
          .catch((err) => { if (!controller.signal.aborted) console.error(err); });
        const params = new URLSearchParams({ sort: "popular", limit: "12", category: nextVideo.category });
        api.videos(params, controller.signal)
          .then((related) => { if (!controller.signal.aborted) setRelatedVideos(related.items.filter((item) => item.id !== nextVideo.id).slice(0, 8)); })
          .catch((err) => { if (!controller.signal.aborted) console.error(err); })
          .finally(() => { if (!controller.signal.aborted) setRelatedLoading(false); });
      })
      .catch((err) => { if (!controller.signal.aborted) setError(errorMessage(err)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [id]);

  useEffect(() => {
    if (!video || (video.processing_status !== "pending" && video.processing_status !== "processing")) return;
    const controller = new AbortController();
    const timer = window.setInterval(() => {
      api.video(id, false, controller.signal)
        .then((nextVideo) => { if (!controller.signal.aborted) setVideo(nextVideo); })
        .catch((err) => { if (!controller.signal.aborted) console.error(err); });
    }, 2500);
    return () => { controller.abort(); window.clearInterval(timer); };
  }, [id, video?.processing_status]);

  const requireUser = () => {
    if (!user) { navigate(`/auth?next=${encodeURIComponent(`/video/${id}`)}`); return false; }
    return true;
  };

  const toggle = async (kind: "like" | "favorite") => {
    if (!video || !requireUser()) return;
    setBusy(kind);
    try {
      const result = kind === "like" ? await api.toggleLike(video.id) : await api.toggleFavorite(video.id);
      setVideo((current) => current ? {
        ...current,
        [kind === "like" ? "liked" : "favorited"]: result.active,
        [kind === "like" ? "likes_count" : "favorites_count"]: current[kind === "like" ? "likes_count" : "favorites_count"] + (result.active ? 1 : -1)
      } : current);
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(""); }
  };

  const toggleAuthorFollow = async () => {
    if (!video || !requireUser()) return;
    setBusy("follow");
    try {
      const next = await api.toggleFollow(video.user_id);
      setAuthorProfile((current) => current ? {
        ...current,
        followed: next.active,
        followers_count: Math.max(0, current.followers_count + (next.active ? 1 : -1))
      } : current);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy("");
    }
  };

  const submitComment = async (event: FormEvent) => {
    event.preventDefault();
    if (!requireUser() || !content.trim()) return;
    setBusy("comment");
    try {
      const created = await api.comment(id, content.trim());
      setComments((current) => [created, ...current]);
      setVideo((current) => current ? { ...current, comments_count: current.comments_count + 1 } : current);
      setContent("");
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(""); }
  };

  const deleteComment = async (comment: Comment) => {
    if (!video || !window.confirm("确定删除这条评论吗？")) return;
    setBusy(`comment-delete-${comment.id}`); setError("");
    try {
      await api.deleteComment(video.id, comment.id);
      setComments((current) => current.filter((item) => item.id !== comment.id));
      setVideo((current) => current ? { ...current, comments_count: Math.max(0, current.comments_count - 1) } : current);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy("");
    }
  };

  const shareVideo = async () => {
    if (!video) return;
    const shareData = { title: video.title, url: window.location.href };
    try {
      if (navigator.share) {
        await navigator.share(shareData);
        setShareStatus("分享面板已打开");
      } else {
        await navigator.clipboard.writeText(shareData.url);
        setShareStatus("链接已复制");
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") return;
      try {
        await navigator.clipboard.writeText(shareData.url);
        setShareStatus("链接已复制");
      } catch {
        setShareStatus("复制失败，请手动复制地址");
      }
    }
    window.setTimeout(() => setShareStatus(""), 2500);
  };

  const submitReport = async (event: FormEvent) => {
    event.preventDefault();
    if (!video || !requireUser()) return;
    setReportBusy(true);
    setError("");
    try {
      await api.reportVideo(video.id, reportReason, reportDetail);
      setReportOpen(false);
      setReportDetail("");
      setShareStatus("举报已提交");
      window.setTimeout(() => setShareStatus(""), 2500);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setReportBusy(false);
    }
  };

  if (loading) return <LoadingBlock label="正在准备播放器" />;
  if (error && !video) return <ErrorBlock message={error} />;
  if (!video) return null;

  return (
    <div className="page watch-page">
      <section className="watch-layout">
        <div className="watch-main">
          <div className="watch-title-row">
            <div><p className="eyebrow">{video.category}</p><h1>{video.title}</h1><div className="watch-meta"><span><Eye size={15} />{formatCount(video.views_count)} 播放</span><span><Clock3 size={15} />{formatDate(video.created_at)}</span>{video.processing_status === "ready" && video.source_width > 0 && <span>{video.source_width} × {video.source_height}</span>}</div></div>
          </div>
          <VideoPlayer video={video} />
          {video.processing_status !== "ready" && (
            <div className={`processing-notice ${video.processing_status}`}>
              <div className="processing-notice-copy">
                <ProcessingBadge video={video} />
                <span>{video.processing_status === "failed" ? (video.processing_error || "媒体处理失败，原始文件仍可播放。") : "原始文件已保存并可播放，后台正在生成自适应清晰度。"}</span>
              </div>
              {(video.processing_status === "pending" || video.processing_status === "processing") && <ProcessingProgress video={video} />}
            </div>
          )}
          <div className="watch-action-row">
            <div className="watch-actions">
              <button className={video.liked ? "active" : ""} disabled={busy === "like"} onClick={() => toggle("like")}><Heart size={19} fill={video.liked ? "currentColor" : "none"} />{formatCount(video.likes_count)}</button>
              <button className={video.favorited ? "active" : ""} disabled={busy === "favorite"} onClick={() => toggle("favorite")}><Bookmark size={19} fill={video.favorited ? "currentColor" : "none"} />{formatCount(video.favorites_count)}</button>
              <button onClick={shareVideo}><Share2 size={19} />分享</button>
              <button onClick={() => { if (requireUser()) setReportOpen(true); }}><Flag size={18} />举报</button>
            </div>
            {shareStatus && <span className="share-feedback" role="status">{shareStatus}</span>}
          </div>
          {error && <p className="inline-error">{error}</p>}
          {reportOpen && <form className="report-form" onSubmit={submitReport}>
            <div className="report-form-heading"><strong>举报视频</strong><button type="button" className="icon-button" onClick={() => setReportOpen(false)} aria-label="关闭举报" title="关闭"><X size={16} /></button></div>
            <label>举报原因<select value={reportReason} onChange={(event) => setReportReason(event.target.value)}><option value="spam">垃圾信息</option><option value="inappropriate">不当内容</option><option value="copyright">版权问题</option><option value="other">其他</option></select></label>
            <label>补充说明<textarea value={reportDetail} onChange={(event) => setReportDetail(event.target.value)} maxLength={1000} placeholder="可以补充时间点或具体原因" /></label>
            <button className="primary-button compact" disabled={reportBusy}><Flag size={15} />{reportBusy ? "提交中..." : "提交举报"}</button>
          </form>}
          <div className="creator-strip">
            <Link to={`/users/${video.user_id}`} aria-label={`查看 ${video.username} 的作者空间`}><Avatar username={video.username} src={video.avatar_url} size="medium" /></Link>
            <div className="creator-strip-copy">
              <Link to={`/users/${video.user_id}`} className="creator-name-link">{video.username}</Link>
              <span>{authorProfile ? `${formatCount(authorProfile.followers_count)} 粉丝` : "创作者"}</span>
              {authorProfile?.bio && <p>{authorProfile.bio}</p>}
            </div>
            {user?.id === video.user_id ? (
              <Link to="/creator" className="secondary-button compact creator-strip-action"><LayoutDashboard size={16} />创作中心</Link>
            ) : (
              <button type="button" className={authorProfile?.followed ? "secondary-button compact follow-button active creator-strip-action" : "primary-button compact creator-strip-action"} disabled={busy === "follow"} onClick={toggleAuthorFollow} aria-pressed={authorProfile?.followed || false}>
                {authorProfile?.followed ? <Check size={16} /> : <UserRound size={16} />}{busy === "follow" ? "处理中..." : authorProfile?.followed ? "已关注" : "关注"}
              </button>
            )}
          </div>
          {video.description && <p className="video-description">{video.description}</p>}
          <section className="comment-section" aria-labelledby="comments-title">
            <div className="comment-heading"><h2 id="comments-title">评论</h2><span>{video.comments_count}</span></div>
            <form className="comment-form" onSubmit={submitComment}>
              <textarea value={content} onChange={(event) => setContent(event.target.value)} maxLength={500} placeholder={user ? "说说你的想法" : "登录后参与讨论"} disabled={!user} />
              <button className="primary-button compact" disabled={!content.trim() || busy === "comment"}><Send size={16} />发布</button>
            </form>
            <div className="comment-list">
              {comments.length ? comments.map((comment) => <div className="comment-item" key={comment.id}><Link to={`/users/${comment.user_id}`} aria-label={`查看 ${comment.username} 的作者空间`}><Avatar username={comment.username} src={comment.avatar_url} size="small" /></Link><div><div className="comment-meta-row"><Link to={`/users/${comment.user_id}`} className="comment-author-link">{comment.username}</Link>{user && (user.id === comment.user_id || user.id === video.user_id) && <button type="button" className="comment-delete-button" disabled={busy === `comment-delete-${comment.id}`} onClick={() => deleteComment(comment)} title="删除评论"><Trash2 size={14} />{busy === `comment-delete-${comment.id}` ? "删除中" : "删除"}</button>}</div><p>{comment.content}</p><time>{formatDate(comment.created_at)}</time></div></div>) : <p className="comment-empty">还没有评论，来聊第一句。</p>}
            </div>
          </section>
        </div>
        <aside className="related-panel" aria-labelledby="related-title">
          <div className="related-heading"><div><p className="eyebrow">继续观看</p><h2 id="related-title">相关推荐</h2></div><Link to={`/popular?category=${encodeURIComponent(video.category)}`}>更多</Link></div>
          {relatedLoading ? (
            <div className="related-list">{Array.from({ length: 5 }, (_, index) => <div className="related-skeleton" key={index}><span /><div><i /><i /></div></div>)}</div>
          ) : relatedVideos.length ? (
            <div className="related-list">{relatedVideos.map((item) => <RelatedVideoCard video={item} key={item.id} />)}</div>
          ) : (
            <div className="related-empty"><Film size={24} /><strong>暂无更多视频</strong><span>同分区的新作品会显示在这里。</span><Link to="/latest">看看最新发布</Link></div>
          )}
        </aside>
      </section>
    </div>
  );
}

export function RelatedVideoCard({ video }: { video: Video }) {
  return (
    <article className="related-video">
      <Link to={`/video/${video.id}`} className="related-cover">
        {video.cover_url ? <img src={video.cover_url} alt="" loading="lazy" /> : <div className="cover-fallback"><Play size={22} fill="currentColor" /></div>}
        <span className="duration">{formatDuration(video.duration_seconds)}</span>
      </Link>
      <div className="related-copy">
        <Link to={`/video/${video.id}`} className="related-title">{video.title}</Link>
        <Link to={`/users/${video.user_id}`} className="related-author"><Avatar username={video.username} src={video.avatar_url} size="small" /><span>{video.username}</span></Link>
        <span><Eye size={13} />{formatCount(video.views_count)} 播放</span>
      </div>
    </article>
  );
}
