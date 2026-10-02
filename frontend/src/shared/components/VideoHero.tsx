import { Eye, MessageCircle, Play } from "lucide-react";
import { Link } from "react-router-dom";
import type { Video } from "../../types";
import { formatCount, formatDate, formatDuration } from "../lib/format";
import { buttonClassName } from "../ui/Button";
import { VideoCover } from "./VideoCover";

export function VideoHero({ video }: { video: Video }) {
  return (
    <article className="gv-video-hero" data-video-id={video.id}>
      <Link className="gv-hero-cover-link" to={`/video/${video.id}`} aria-label={`播放 ${video.title}`}><VideoCover src={video.cover_url} eager /></Link>
      <span className="gv-hero-shade" aria-hidden="true" />
      <span className="duration">{formatDuration(video.duration_seconds)}</span>
      <div className="gv-hero-copy">
        <p className="gv-hero-eyebrow">最新发布{video.category && <span> / {video.category}</span>}</p>
        <h2><Link to={`/video/${video.id}`} title={video.title}>{video.title}</Link></h2>
        {video.description && <p className="gv-hero-description">{video.description}</p>}
        <Link to={`/users/${video.user_id}`} className="gv-hero-creator" title={video.username}>{video.username}</Link>
        <div className="gv-hero-meta"><span><Eye size={14} />{formatCount(video.views_count)} 次播放</span><span><MessageCircle size={14} />{formatCount(video.comments_count)} 条评论</span><time dateTime={video.created_at}>{formatDate(video.created_at)}</time></div>
        <Link to={`/video/${video.id}`} className={buttonClassName("primary")}><Play size={16} fill="currentColor" />立即观看</Link>
      </div>
    </article>
  );
}
