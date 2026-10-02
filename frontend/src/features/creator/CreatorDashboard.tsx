import { useEffect, useState } from "react";
import { Clock3, Film, Upload, UserRound } from "lucide-react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { formatCount, formatDate } from "../../shared/lib/format";
import { VideoCover } from "../../shared/components/VideoCover";
import { StudioEmptyState } from "../../shared/components/StudioEmptyState";
import { ProcessingBadge } from "../../shared/components/Processing";
import { LoadingBlock, ErrorBlock } from "../../shared/components/Feedback";
import { visibilityLabels } from "../../shared/lib/visibility";
import type { CreatorStats, User } from "../../types";

export function CreatorDashboard({ user }: { user: User }) {
  const [stats, setStats] = useState<CreatorStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api.creatorStats()
      .then(setStats)
      .catch((err) => setError(errorMessage(err)))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingBlock label="正在整理创作数据" />;
  if (error) return <ErrorBlock message={error} />;
  if (!stats) return null;

  const number = (value: number) => new Intl.NumberFormat("zh-CN").format(value);
  const engagement = [
    { label: "获赞", value: stats.likes_count },
    { label: "收藏", value: stats.favorites_count },
    { label: "评论", value: stats.comments_count }
  ];
  const visibility = [
    { label: visibilityLabels.public, value: stats.public_count, tone: "public" },
    { label: visibilityLabels.unlisted, value: stats.unlisted_count, tone: "unlisted" },
    { label: visibilityLabels.private, value: stats.private_count, tone: "private" }
  ];

  return (
    <div className="page gv-studio-dashboard">
      <section className="page-heading heading-row gv-studio-heading">
        <div><p className="eyebrow">STUDIO / OVERVIEW</p><h1>作品与观众概览</h1><p className="gv-studio-creator-name">{user.username} <span>· 累计创作数据</span></p></div>
        <div className="gv-studio-heading-actions"><Link to={`/users/${user.id}`} className="gv-studio-text-link"><UserRound size={16} />个人空间</Link><Link to="/me/videos" className="gv-studio-text-link"><Film size={16} />管理投稿</Link><Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link></div>
      </section>
      <section className="gv-studio-performance" aria-label="创作表现">
        <div className="gv-studio-total"><span>全部投稿</span><strong>{number(stats.videos_count)}</strong><small>作品</small></div>
        <dl className="gv-studio-reach"><div><dt>总播放</dt><dd>{number(stats.views_count)}</dd></div><div><dt>关注者</dt><dd>{number(stats.followers_count)}</dd></div></dl>
        <dl className="gv-studio-engagement">{engagement.map((item) => <div key={item.label}><dt>{item.label}</dt><dd>{number(item.value)}</dd></div>)}</dl>
      </section>
      <section className="gv-studio-status" aria-labelledby="visibility-title">
        <div><p className="eyebrow">CONTENT STATUS</p><h2 id="visibility-title">可见范围</h2></div>
        <dl>{visibility.map((item) => <div key={item.tone}><dt><span className={`visibility-dot ${item.tone}`} />{item.label}</dt><dd>{number(item.value)}</dd></div>)}</dl>
        <p className="gv-studio-processing"><Clock3 size={16} /><span>{stats.processing_count > 0 ? <><strong>{number(stats.processing_count)}</strong> 条投稿正在处理</> : "当前没有正在处理的投稿"}<small>处理状态独立于可见范围</small></span></p>
      </section>
      {stats.videos_count === 0 ? <StudioEmptyState /> : <section className="gv-studio-recent" aria-labelledby="recent-videos-title">
        <header><div><p className="eyebrow">RECENT CONTENT</p><h2 id="recent-videos-title">最近投稿</h2></div><Link to="/me/videos" className="gv-studio-text-link">管理全部内容 ↗</Link></header>
        {stats.recent_videos.length ? <div className="gv-studio-recent-list">{stats.recent_videos.map((video) => <article className="gv-studio-recent-row" key={video.id}>
          <Link to={`/video/${video.id}`} className="gv-content-cover" aria-label={`观看${video.title}`}><VideoCover src={video.cover_url} /></Link>
          <div className="gv-studio-recent-copy"><Link className="gv-content-title" to={`/video/${video.id}`} title={video.title}>{video.title}</Link><span>{formatDate(video.created_at)} · {formatCount(video.views_count)} 播放</span></div>
          <span className={`visibility-badge ${video.visibility}`}>{visibilityLabels[video.visibility]}</span><ProcessingBadge video={video} />
        </article>)}</div> : <p className="gv-studio-recent-empty">暂无最近投稿。<Link to="/me/videos">查看全部内容</Link></p>}
      </section>}
    </div>
  );
}
