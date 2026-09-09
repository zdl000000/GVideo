import { useEffect, useState } from "react";
import { Clock3, Compass, Home, Sparkles } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { pageFrom } from "../../shared/lib/format";
import { EmptyState, ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
import type { VideoPage as VideoPageData } from "../../types";

export function PopularPage() {
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
    const params = new URLSearchParams({ sort: "popular", page: String(page), page_size: "36" });
    if (query) params.set("q", query);
    if (category) params.set("category", category);
    api.videos(params, controller.signal).then(setResult).catch((err) => { if (!controller.signal.aborted) setError(errorMessage(err)); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [query, category, page]);

  const setCategory = (value: string) => {
    const next = new URLSearchParams(searchParams);
    value ? next.set("category", value) : next.delete("category");
    next.delete("sort");
    next.delete("page");
    setSearchParams(next);
  };
  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };
  const latestParams = new URLSearchParams(searchParams);
  latestParams.delete("sort");
  latestParams.delete("page");
  const latestTarget = `/latest${latestParams.size ? `?${latestParams}` : ""}`;

  return (
    <div className="page popular-page">
      <section className="popular-head">
        <div>
          <p className="eyebrow">热门发现</p>
          <h1>{query ? `“${query}”的热门内容` : category ? `${category}热门内容` : "此刻最受关注"}</h1>
          <p>按播放量与点赞综合排序，发现社区里更受关注的作品。</p>
        </div>
        <div className="popular-rule" aria-label="热门排序规则">
          <Sparkles size={20} />
          <span><strong>热度榜</strong>播放与点赞综合排序</span>
        </div>
      </section>

      <section className="content-heading popular-title-row">
        <div><h2>热门榜单</h2><span>共 {result.total} 条视频</span></div>
      </section>

      <section className="filter-row" aria-label="热门视频筛选">
        <div className="category-scroller">
          <button className={!category ? "active" : ""} onClick={() => setCategory("")}>全部</button>
          {categories.map((item) => <button className={category === item ? "active" : ""} key={item} onClick={() => setCategory(item)}>{item}</button>)}
        </div>
        <div className="segmented-control">
          <Link to={latestTarget}><Clock3 size={15} />最新</Link>
          <Link className="active" to={`/popular${searchParams.size ? `?${searchParams}` : ""}`}><Sparkles size={15} />热门</Link>
        </div>
      </section>

      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : videos.length ? (
        <><section className="video-grid popular-grid">
          {videos.map((video, index) => { const rank = (result.page - 1) * result.page_size + index + 1; return <div className="ranked-video" key={video.id}><span className={`rank-number ${rank <= 3 ? "top" : ""}`}>{String(rank).padStart(2, "0")}</span><VideoCard video={video} /></div>; })}
        </section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Compass size={28} />} title="暂无热门内容" text={query || category ? "换个关键词或分区继续看看。" : "作品获得播放和点赞后，会出现在这里。"} action={<Link to="/" className="secondary-button"><Home size={17} />返回首页</Link>} />
      )}
    </div>
  );
}
