import { useEffect, useState } from "react";
import { Clock3, Eye, Film, MessageCircle, Play, Sparkles, Upload } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { formatCount, formatDuration, pageFrom } from "../../shared/lib/format";
import { EmptyState, ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
import type { VideoPage as VideoPageData } from "../../types";

export function HomePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 36, total: 0, has_next: false });
  const [categories, setCategories] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const query = searchParams.get("q") || "";
  const category = searchParams.get("category") || "";
  const page = pageFrom(searchParams);
  const videos = result.items;

  useEffect(() => {
    api.categories().then(setCategories).catch(console.error);
  }, []);

  useEffect(() => {
    setLoading(true);
    setError("");
    const controller = new AbortController();
    const params = new URLSearchParams({ sort: "latest", page: String(page), page_size: "36" });
    if (query) params.set("q", query);
    if (category) params.set("category", category);
    api.videos(params, controller.signal).then(setResult).catch((err) => { if (!controller.signal.aborted) setError(errorMessage(err)); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [query, category, page]);

  const featured = page === 1 ? videos.slice(0, 5) : [];
  const leadVideo = featured[0];
  const secondaryVideos = featured.slice(1);
  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    value ? next.set(key, value) : next.delete(key);
    next.delete("page");
    setSearchParams(next);
  };
  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };
  const popularParams = new URLSearchParams(searchParams);
  popularParams.delete("sort");
  popularParams.delete("page");
  const popularTarget = `/popular${popularParams.size ? `?${popularParams}` : ""}`;
  const latestTarget = `/latest${popularParams.size ? `?${popularParams}` : ""}`;

  return (
    <div className="page home-page">
      {query || category ? (
        <section className="discovery-head compact-heading">
          <div><p className="eyebrow">内容检索</p><h1>{query ? `“${query}”的搜索结果` : category}</h1></div>
          <div className="head-stat"><strong>{result.total}</strong><span>条内容</span></div>
        </section>
      ) : leadVideo ? (
        <section className={`featured-showcase ${secondaryVideos.length ? "" : "single"} ${secondaryVideos.length === 1 ? "sparse" : ""}`} aria-label="最新发布视频">
          <article className="featured-lead">
            <Link to={`/video/${leadVideo.id}`} className="featured-click-target" aria-label={`播放 ${leadVideo.title}`} />
            {leadVideo.cover_url ? <img src={leadVideo.cover_url} alt="" /> : <div className="cover-fallback"><Play size={42} fill="currentColor" /></div>}
            <span className="featured-shade" />
            <span className="featured-badge">最新发布</span>
            <span className="featured-lead-copy">
              <Link to={`/video/${leadVideo.id}`} className="featured-lead-title">{leadVideo.title}</Link>
              <span><Link to={`/users/${leadVideo.user_id}`} className="featured-author-link">{leadVideo.username}</Link><span><Eye size={14} />{formatCount(leadVideo.views_count)}</span><span><MessageCircle size={14} />{formatCount(leadVideo.comments_count)}</span></span>
            </span>
            <span className="duration">{formatDuration(leadVideo.duration_seconds)}</span>
          </article>
          {secondaryVideos.length > 0 && (
            <div className="featured-secondary">
              {secondaryVideos.map((video) => (
                <article className="featured-mini" key={video.id}>
                  <Link to={`/video/${video.id}`} className="featured-mini-cover">
                    {video.cover_url ? <img src={video.cover_url} alt="" loading="lazy" /> : <div className="cover-fallback"><Play size={24} fill="currentColor" /></div>}
                    <span className="featured-mini-stats"><span><Eye size={13} />{formatCount(video.views_count)}</span><span><MessageCircle size={13} />{formatCount(video.comments_count)}</span></span>
                    <span className="duration">{formatDuration(video.duration_seconds)}</span>
                  </Link>
                  <Link to={`/video/${video.id}`} className="featured-mini-title">{video.title}</Link>
                  <span className="featured-mini-author"><Link to={`/users/${video.user_id}`}>{video.username}</Link> · {video.category}</span>
                </article>
              ))}
            </div>
          )}
        </section>
      ) : null}

      <section className="content-heading">
        <div><h2>{query || category ? "筛选结果" : "最新视频"}</h2><span>共 {result.total} 条视频</span></div>
      </section>

      <section className="filter-row" aria-label="视频筛选">
        <div className="category-scroller">
          <button className={!category ? "active" : ""} onClick={() => setFilter("category", "")}>全部</button>
          {categories.map((item) => <button className={category === item ? "active" : ""} key={item} onClick={() => setFilter("category", item)}>{item}</button>)}
        </div>
        <div className="segmented-control">
          <Link to={latestTarget}><Clock3 size={15} />最新</Link>
          <Link to={popularTarget}><Sparkles size={15} />热门</Link>
        </div>
      </section>

      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : videos.length ? (
        <><section className="video-grid">{videos.map((video) => <VideoCard video={video} key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Film size={28} />} title="这里还没有视频" text={query || category ? "换个关键词或分类继续找找。" : "成为第一个发布作品的人。"} action={<Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link>} />
      )}
    </div>
  );
}
