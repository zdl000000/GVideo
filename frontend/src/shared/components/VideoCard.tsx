import type { ReactNode } from "react";
import { Eye, MessageCircle, Play } from "lucide-react";
import { Link } from "react-router-dom";
import type { Video } from "../../types";
import { formatCount, formatDate, formatDuration } from "../lib/format";
import { Avatar } from "./Avatar";
import { VideoCover } from "./VideoCover";

type CardProps = { video: Video; action?: ReactNode; creatorFirst?: boolean } & (
  { variant: "ranked"; rank: number } | { variant?: "standard" | "editorial" | "compact"; rank?: never }
);

export function VideoCard({ video, variant, rank, action, creatorFirst = false }: CardProps) {
  // Calls outside Phase 2 keep their existing density until their own page migration.
  const legacy = !variant;
  const creator = <div className="video-author"><Link to={`/users/${video.user_id}`} className="video-author-link"><Avatar username={video.username} src={video.avatar_url} size={creatorFirst ? "medium" : "small"} /><span>{video.username}</span></Link>{legacy && video.category && <span className="category-label">{video.category}</span>}</div>;
  return (
    <article className={`video-card ${variant ? `gv-video-card gv-video-card--${variant}` : ""} ${creatorFirst ? "gv-video-card--creator-first" : ""} ${rank && rank <= 3 ? "gv-video-card--top" : ""}`.trim()} data-video-id={video.id}>
      {variant === "ranked" && <span className="gv-rank" aria-label={`第 ${rank} 名`}>{String(rank).padStart(2, "0")}</span>}
      <Link to={`/video/${video.id}`} className="cover-link" aria-label={`播放 ${video.title}`}>
        <VideoCover src={video.cover_url} />
        <span className="duration">{formatDuration(video.duration_seconds)}</span>
        <span className="play-overlay" aria-hidden="true"><Play size={22} fill="currentColor" /></span>
      </Link>
      <div className="video-card-body">
        {creatorFirst && creator}
        {variant === "editorial" && !creatorFirst && video.category && <span className="gv-card-eyebrow">{video.category}</span>}
        <Link to={`/video/${video.id}`} className="video-title" title={video.title}>{video.title}</Link>
        {variant === "editorial" && video.description && <p className="gv-card-description">{video.description}</p>}
        {!creatorFirst && creator}
        <div className="video-stats"><span>{legacy && <Eye size={14} />}{formatCount(video.views_count)}{!legacy && " 次播放"}</span>{legacy && <span><MessageCircle size={14} />{formatCount(video.comments_count)}</span>}<time dateTime={video.created_at}>{formatDate(video.created_at)}</time></div>
        {action && <div className="gv-card-action">{action}</div>}
      </div>
    </article>
  );
}
