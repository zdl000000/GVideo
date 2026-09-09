import { useEffect, useState } from "react";
import { Clock3, Sparkles, Upload } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { pageFrom } from "../../shared/lib/format";
import { EmptyState, ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
import type { VideoPage as VideoPageData } from "../../types";

export function LatestPage() {
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
  const popularParams = new URLSearchParams(searchParams);
  popularParams.delete("sort");
  popularParams.delete("page");
  const popularTarget = `/popular${popularParams.size ? `?${popularParams}` : ""}`;

  return (
    <div className="page latest-page">
      <section className="latest-head">
        <div>
          <p className="eyebrow">最新发布</p>
          <h1>{query ? `“${query}”的最新内容` : category ? `${category}最新内容` : "刚刚与大家见面"}</h1>
          <p>按发布时间倒序排列，不错过社区里的新作品。</p>
        </div>
        <div className="latest-rule" aria-label="最新排序规则">
          <Clock3 size={20} />
          <span><strong>更新时间线</strong>最近发布优先展示</span>
        </div>
      </section>

      <section className="content-heading latest-title-row">
        <div><h2>最新视频</h2><span>共 {result.total} 条视频</span></div>
      </section>

      <section className="filter-row" aria-label="最新视频筛选">
        <div className="category-scroller">
          <button className={!category ? "active" : ""} onClick={() => setCategory("")}>全部</button>
          {categories.map((item) => <button className={category === item ? "active" : ""} key={item} onClick={() => setCategory(item)}>{item}</button>)}
        </div>
        <div className="segmented-control">
          <Link className="active" to={`/latest${searchParams.size ? `?${searchParams}` : ""}`}><Clock3 size={15} />最新</Link>
          <Link to={popularTarget}><Sparkles size={15} />热门</Link>
        </div>
      </section>

      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : videos.length ? (
        <><section className="video-grid">{videos.map((video) => <VideoCard video={video} key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Clock3 size={28} />} title="暂无最新内容" text={query || category ? "换个关键词或分区继续看看。" : "新作品发布后，会按时间出现在这里。"} action={<Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link>} />
      )}
    </div>
  );
}
