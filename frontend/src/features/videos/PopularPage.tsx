import { Compass, Home } from "lucide-react";
import { Link } from "react-router-dom";
import { EmptyState, ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
import { CategoryFilter, SectionHeader, SortSwitch } from "../../shared/components/DiscoveryControls";
import { useDiscoveryFeed } from "./useDiscoveryFeed";

export function PopularPage() {
  const feed = useDiscoveryFeed("popular");
  const { result, query, category, loading, error } = feed;
  const offset = (result.page - 1) * result.page_size;
  const leaders = offset === 0 ? result.items.slice(0, 3) : [];
  const rest = result.items.slice(leaders.length);
  return (
    <div className="page gv-discovery-page popular-page">
      <header className="gv-discovery-header"><div><p className="eyebrow">POPULAR / 热门发现</p><h1>{query ? `“${query}”的热门内容` : category ? `${category}热门内容` : "此刻最受关注"}</h1><p>按播放量与点赞综合排序，发现社区里更受关注的作品。</p></div><span className="gv-content-count" role="status">{loading ? "正在加载榜单" : error ? "内容暂时无法加载" : `共 ${result.total} 条视频`}</span></header>
      <section className="filter-row" aria-label="热门视频筛选"><CategoryFilter categories={feed.categories} value={category} onChange={feed.setCategory} /><SortSwitch params={feed.params} active="popular" /></section>
      {feed.categoryError && <p className="gv-filter-notice" role="status">{feed.categoryError}<button onClick={feed.retry}>重试</button></p>}
      <SectionHeader title="热门榜单" detail="播放与点赞综合排序" />
      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : result.items.length ? <>
        {leaders.length > 0 && <section className={`gv-popular-leaders ${leaders.length === 1 ? "gv-popular-leaders--single" : ""}`} aria-label="榜单前三名">{leaders.map((video, index) => <VideoCard video={video} variant="ranked" rank={index + 1} key={video.id} />)}</section>}
        {rest.length > 0 && <section className="gv-ranked-list" aria-label="完整热门榜单">{rest.map((video, index) => <VideoCard video={video} variant="ranked" rank={offset + leaders.length + index + 1} key={video.id} />)}</section>}
        <Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={feed.setPage} />
      </> : <EmptyState icon={<Compass size={28} />} title="暂无热门内容" text={query || category ? "换个关键词或分区继续看看。" : "作品获得播放和点赞后，会出现在这里。"} action={<Link to="/" className="secondary-button"><Home size={17} />返回首页</Link>} />}
    </div>
  );
}
