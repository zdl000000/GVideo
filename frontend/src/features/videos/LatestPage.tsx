import { Clock3, Upload } from "lucide-react";
import { Link } from "react-router-dom";
import { EmptyState, ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
import { CategoryFilter, SectionHeader, SortSwitch } from "../../shared/components/DiscoveryControls";
import { useDiscoveryFeed } from "./useDiscoveryFeed";

export function LatestPage() {
  const feed = useDiscoveryFeed("latest");
  const { result, query, category, loading, error } = feed;
  return (
    <div className="page gv-discovery-page latest-page">
      <header className="gv-discovery-header"><div><p className="eyebrow">LATEST / 最新发布</p><h1>{query ? `“${query}”的最新内容` : category ? `${category}最新内容` : "刚刚与大家见面"}</h1><p>按发布时间倒序排列，不错过社区里的新作品。</p></div><span className="gv-content-count" role="status">{loading ? "正在加载内容" : error ? "内容暂时无法加载" : `共 ${result.total} 条视频`}</span></header>
      <section className="filter-row" aria-label="最新视频筛选"><CategoryFilter categories={feed.categories} value={category} onChange={feed.setCategory} /><SortSwitch params={feed.params} active="latest" /></section>
      {feed.categoryError && <p className="gv-filter-notice" role="status">{feed.categoryError}<button onClick={feed.retry}>重试</button></p>}
      <SectionHeader title="最新视频" detail="最近发布优先展示" />
      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : result.items.length ? <><section className="video-grid">{result.items.map((video) => <VideoCard video={video} variant="standard" key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={feed.setPage} /></> : <EmptyState icon={<Clock3 size={28} />} title="暂无最新内容" text={query || category ? "换个关键词或分区继续看看。" : "新作品发布后，会按时间出现在这里。"} action={<Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link>} />}
    </div>
  );
}
