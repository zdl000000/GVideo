import { useEffect, useState } from "react";
import { Bookmark, Clock3, Eye, Film, Heart, MessageCircle, Play, Upload, UserRound } from "lucide-react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { formatCount, formatDate } from "../../shared/lib/format";
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

  const performance = [
    { label: "总播放", value: stats.views_count, icon: <Eye size={18} /> },
    { label: "获赞", value: stats.likes_count, icon: <Heart size={18} /> },
    { label: "收藏", value: stats.favorites_count, icon: <Bookmark size={18} /> },
    { label: "评论", value: stats.comments_count, icon: <MessageCircle size={18} /> }
  ];
  const visibility = [
    { label: visibilityLabels.public, value: stats.public_count, tone: "public" },
    { label: visibilityLabels.unlisted, value: stats.unlisted_count, tone: "unlisted" },
    { label: visibilityLabels.private, value: stats.private_count, tone: "private" }
  ];

  return (
    <div className="page creator-dashboard-page">
      <section className="page-heading heading-row">
        <div><p className="eyebrow">创作者中心</p><h1>作品与观众概览</h1><p>集中查看投稿状态和累计互动，继续管理你的内容。</p></div>
        <div className="dashboard-heading-actions"><Link to={`/users/${user.id}`} className="secondary-button"><UserRound size={17} />个人空间</Link><Link to="/me/videos" className="secondary-button"><Film size={17} />管理投稿</Link><Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link></div>
      </section>

      <section className="dashboard-performance" aria-label="创作表现">
        <div className="dashboard-primary-stat"><span>全部投稿</span><strong>{formatCount(stats.videos_count)}</strong><small>{formatCount(stats.followers_count)} 位关注者</small></div>
        {performance.map((item) => <div className="dashboard-stat" key={item.label}><span>{item.icon}{item.label}</span><strong>{formatCount(item.value)}</strong></div>)}
      </section>

      <div className="dashboard-columns">
        <section className="dashboard-section" aria-labelledby="recent-videos-title">
          <header><div><p className="eyebrow">最近投稿</p><h2 id="recent-videos-title">继续完善作品</h2></div><Link to="/me/videos">查看全部</Link></header>
          {stats.recent_videos.length ? (
            <div className="dashboard-video-list">
              {stats.recent_videos.map((video) => (
                <article className="dashboard-video-row" key={video.id}>
                  <Link to={`/video/${video.id}`} className="dashboard-video-cover">
                    {video.cover_url ? <img src={video.cover_url} alt="" /> : <div className="cover-fallback"><Play size={20} fill="currentColor" /></div>}
                  </Link>
                  <div className="dashboard-video-copy"><Link to={`/video/${video.id}`}>{video.title}</Link><span>{formatDate(video.created_at)} · {formatCount(video.views_count)} 播放</span></div>
                  <div className="dashboard-video-status"><span className={`visibility-badge ${video.visibility}`}>{visibilityLabels[video.visibility]}</span><ProcessingBadge video={video} /></div>
                </article>
              ))}
            </div>
          ) : (
            <div className="dashboard-empty"><Film size={24} /><span>还没有投稿</span><Link to="/upload">发布第一条作品</Link></div>
          )}
        </section>

        <section className="dashboard-section visibility-summary" aria-labelledby="visibility-title">
          <header><div><p className="eyebrow">内容状态</p><h2 id="visibility-title">可见范围</h2></div></header>
          <dl>{visibility.map((item) => <div key={item.tone}><dt><span className={`visibility-dot ${item.tone}`} />{item.label}</dt><dd>{formatCount(item.value)}</dd></div>)}</dl>
          <p><Clock3 size={15} />{stats.processing_count > 0 ? `${stats.processing_count} 条投稿正在处理` : "当前没有正在处理的投稿"}</p>
        </section>
      </div>
    </div>
  );
}
