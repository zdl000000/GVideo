import { useEffect, useState } from "react";
import { Compass, Users } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { pageFrom } from "../../shared/lib/format";
import { EmptyState, ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
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
      .then(setResult)
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
    <div className="page following-page">
      <section className="following-head">
        <div><p className="eyebrow">关注动态</p><h1>关注的人，刚刚发布</h1><p>按发布时间查看你关注的创作者新作。</p></div>
        <div className="following-rule"><Users size={20} /><span><strong>{result.total} 条动态</strong>只展示已关注作者的投稿</span></div>
      </section>
      <section className="content-heading following-title-row"><div><h2>最新投稿</h2><span>服务端分页 · 每页 24 条</span></div></section>
      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : result.items.length ? (
        <><section className="video-grid">{result.items.map((video) => <VideoCard video={video} key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Users size={28} />} title="关注动态还是空的" text="去视频页或作者空间关注喜欢的创作者，新投稿会出现在这里。" action={<Link to="/popular" className="primary-button"><Compass size={17} />发现创作者</Link>} />
      )}
    </div>
  );
}
