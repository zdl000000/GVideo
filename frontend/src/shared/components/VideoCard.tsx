import { Eye, MessageCircle, Play } from "lucide-react";
import { Link } from "react-router-dom";
import type { Video } from "../../types";
import { formatCount, formatDate, formatDuration } from "../lib/format";
import { Avatar } from "./Avatar";

export function VideoCard({ video }: { video: Video }) {
  return (
    <article className="video-card">
      <Link to={`/video/${video.id}`} className="cover-link">
        {video.cover_url ? <img src={video.cover_url} alt="" loading="lazy" /> : <div className="cover-fallback"><Play size={28} fill="currentColor" /></div>}
        <span className="duration">{formatDuration(video.duration_seconds)}</span>
        <span className="play-overlay"><Play size={22} fill="currentColor" /></span>
      </Link>
      <div className="video-card-body">
        <Link to={`/video/${video.id}`} className="video-title">{video.title}</Link>
        <div className="video-author"><Link to={`/users/${video.user_id}`} className="video-author-link"><Avatar username={video.username} src={video.avatar_url} size="small" /><span>{video.username}</span></Link><span className="category-label">{video.category}</span></div>
        <div className="video-stats"><span><Eye size={14} />{formatCount(video.views_count)}</span><span><MessageCircle size={14} />{formatCount(video.comments_count)}</span><time>{formatDate(video.created_at)}</time></div>
      </div>
    </article>
  );
}
