import { Eye, Heart, MessageCircle } from "lucide-react";
import { Link } from "react-router-dom";
import { ProcessingBadge, ProcessingProgress } from "../../shared/components/Processing";
import { VideoCover } from "../../shared/components/VideoCover";
import { formatCount, formatDate, formatDuration, formatFileSize } from "../../shared/lib/format";
import { visibilityLabels } from "../../shared/lib/visibility";
import type { Video } from "../../types";
import { ContentActionMenu } from "./ContentActionMenu";

interface ContentRowProps {
  video: Video;
  busy: string;
  onEdit: (trigger: HTMLButtonElement) => void;
  onSubtitles: (trigger: HTMLButtonElement) => void;
  onDelete: (trigger: HTMLButtonElement) => void;
  onRetry: () => void;
}

export function ContentRow({ video, ...actions }: ContentRowProps) {
  const processing = video.processing_status === "pending" || video.processing_status === "processing";
  return <article className={`gv-content-row gv-content-row--${video.processing_status}`} aria-label={video.title}>
    <div className="gv-content-identity">
      <Link to={`/video/${video.id}`} className="gv-content-cover" aria-label={`观看${video.title}`}><VideoCover src={video.cover_url} /><span className="duration">{formatDuration(video.duration_seconds)}</span></Link>
      <div className="gv-content-copy"><Link className="gv-content-title" to={`/video/${video.id}`} title={video.title}>{video.title}</Link><p>{video.description || "暂未填写视频简介"}</p><span>{video.category} <i>·</i> {formatFileSize(video.size_bytes)}</span></div>
    </div>
    <div className="gv-content-state"><ProcessingBadge video={video} /></div>
    <div className="gv-content-visibility"><span className={`visibility-badge ${video.visibility}`}>{visibilityLabels[video.visibility]}</span></div>
    <div className="gv-content-performance"><strong><Eye size={14} />{formatCount(video.views_count)}<span>播放</span></strong><small><span><Heart size={12} />{formatCount(video.likes_count)}</span><span><MessageCircle size={12} />{formatCount(video.comments_count)}<span className="sr-only">评论</span></span></small></div>
    <time className="gv-content-date" dateTime={video.created_at}>{formatDate(video.created_at)}</time>
    <ContentActionMenu video={video} {...actions} />
    {processing && <div className="gv-content-progress"><ProcessingProgress video={video} compact /></div>}
    {video.processing_status === "failed" && <p className="gv-content-failure">{video.processing_error || "媒体处理失败，可以手动重新转码"}</p>}
  </article>;
}
