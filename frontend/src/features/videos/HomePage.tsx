import { useEffect, useState } from "react";
import { Film, Upload } from "lucide-react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { EmptyState, ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
import { VideoHero } from "../../shared/components/VideoHero";
import { CategoryFilter, SectionHeader, SortSwitch } from "../../shared/components/DiscoveryControls";
import { useDiscoveryFeed } from "./useDiscoveryFeed";
import type { Video } from "../../types";

export function HomePage() {
  const feed = useDiscoveryFeed("latest");
  const { result, query, category, page, loading, error } = feed;
  const filtered = Boolean(query || category);
  const discovery = !filtered && page === 1;
  const [popular, setPopular] = useState<Video[]>([]);
  const [popularLoading, setPopularLoading] = useState(false);
  const [popularError, setPopularError] = useState("");
  const [popularRevision, setPopularRevision] = useState(0);

  useEffect(() => {
    if (!discovery) return;
    const controller = new AbortController();
    setPopular([]);
    setPopularLoading(true);
    setPopularError("");
    api.videos(new URLSearchParams({ sort: "popular", page: "1", page_size: "12" }), controller.signal)
      .then((data) => { if (!controller.signal.aborted) setPopular(data.items); })
      .catch((err) => { if (!controller.signal.aborted) setPopularError(errorMessage(err)); })
      .finally(() => { if (!controller.signal.aborted) setPopularLoading(false); });
    return () => controller.abort();
  }, [discovery, popularRevision]);

  const videos = result.items.filter((video, index, items) => items.findIndex((item) => item.id === video.id) === index);
  const featured = discovery ? videos.filter((video) => video.processing_status === "ready").slice(0, 4) : [];
  const featuredIDs = new Set(featured.map((video) => video.id));
  const latest = videos.filter((video) => !featuredIDs.has(video.id));
  const visibleIDs = new Set(videos.map((video) => video.id));
  const popularPreview = popular.map((video, index) => ({ video, rank: index + 1 })).filter(({ video }, index, items) => !visibleIDs.has(video.id) && items.findIndex((item) => item.video.id === video.id) === index).slice(0, 3);

  return (
    <div className={`page gv-discovery-page home-page ${filtered ? "gv-results-mode" : ""}`}>
      <header className="gv-discovery-header">
        <div><p className="eyebrow">{query ? "SEARCH / 搜索" : category ? "CATEGORY / 分类" : "WATCH / 发现"}</p><h1>{query ? `“${query}”的搜索结果` : category || "发现好视频"}</h1></div>
        <span className="gv-content-count" role="status">{loading ? "正在加载内容" : error ? "内容暂时无法加载" : `共 ${result.total} 条视频`}</span>
      </header>
      <section className="filter-row" aria-label="视频筛选"><CategoryFilter categories={feed.categories} value={category} onChange={feed.setCategory} /><SortSwitch params={feed.params} active="latest" /></section>
      {feed.categoryError && <p className="gv-filter-notice" role="status">{feed.categoryError}<button onClick={feed.retry}>重试</button></p>}
      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : videos.length ? <>
        {featured.length > 0 && <section className={`gv-home-stories ${featured.length === 1 ? "gv-home-stories--single" : ""}`} aria-label="最新发布视频"><VideoHero video={featured[0]} />{featured.length > 1 && <div className="gv-secondary-stories"><p className="gv-stories-label">继续发现 <span> / LATEST</span></p>{featured.slice(1).map((video) => <VideoCard key={video.id} video={video} variant="compact" />)}</div>}</section>}
        {latest.length > 0 && <section aria-label={filtered ? "筛选结果" : "最新视频"}><SectionHeader title={filtered ? "筛选结果" : "最新视频"} detail={filtered ? undefined : "按发布时间排序"} to={filtered ? undefined : "/latest"} /><div className={discovery ? "gv-latest-mosaic" : "video-grid"}>{latest.map((video, index) => <VideoCard key={video.id} video={video} variant={discovery && index === 0 && latest.length > 1 ? "editorial" : "standard"} />)}</div></section>}
        <Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={feed.setPage} />
        {discovery && (popularLoading || popularError || popularPreview.length > 0) && <section className="gv-popular-preview" aria-label="热门预览"><SectionHeader title="热门发现" detail="播放与点赞综合排序" to="/popular" />{popularLoading ? <p role="status" className="gv-preview-status">正在加载热门内容…</p> : popularError ? <div className="gv-preview-status" role="status"><p>热门内容暂时无法加载：{popularError}</p><button type="button" className="secondary-button" onClick={() => setPopularRevision((value) => value + 1)}>重试热门内容</button></div> : <div className="gv-ranked-list">{popularPreview.map(({ video, rank }) => <VideoCard key={video.id} video={video} variant="ranked" rank={rank} />)}</div>}</section>}
      </> : <EmptyState icon={<Film size={28} />} title="这里还没有视频" text={filtered ? "换个关键词或分类继续找找。" : "成为第一个发布作品的人。"} action={<Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link>} />}
    </div>
  );
}
