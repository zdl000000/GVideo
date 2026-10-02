import { useEffect, useState } from "react";
import { Compass, Users } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { pageFrom } from "../../shared/lib/format";
import { ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { WatchEmptyState } from "../../shared/components/WatchEmptyState";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
import { SectionHeader } from "../../shared/components/DiscoveryControls";
import type { VideoPage as VideoPageData } from "../../types";

export function FollowingPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 24, total: 0, has_next: false });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const page = pageFrom(searchParams);

  useEffect(() => {
    setLoading(true);
    setError("");
    const controller = new AbortController();
    api.followingVideos(new URLSearchParams({ page: String(page), page_size: "24" }), controller.signal)
      .then((data) => { if (!controller.signal.aborted) setResult(data); })
      .catch((err) => { if (!controller.signal.aborted) setError(errorMessage(err)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [page]);

  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  return (
    <div className="page gv-discovery-page following-page">
      <header className="gv-discovery-header"><div><p className="eyebrow">FOLLOWING / 关注动态</p><h1>来自关注的创作者</h1><p>按发布时间查看你关注的创作者新作。</p></div><span className="gv-content-count" role="status">{loading ? "正在加载动态" : error ? "内容暂时无法加载" : `${result.total} 条动态`}</span></header>
      <SectionHeader title="最新投稿" detail="你关注的创作者 · 最近发布优先" />
      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : result.items.length ? <><section className="gv-following-feed" aria-label="关注时间流">{result.items.map((video) => <VideoCard video={video} variant="editorial" creatorFirst key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></> : <WatchEmptyState eyebrow="01 / FOLLOWING" icon={<Users size={28} />} title="关注动态还是空的" text="去视频页或作者空间关注喜欢的创作者，新投稿会出现在这里。" action={<Link to="/popular" className="primary-button"><Compass size={17} />发现创作者</Link>} />}
    </div>
  );
}
