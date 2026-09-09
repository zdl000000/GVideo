import { useEffect, useState } from "react";
import { Bookmark, Compass } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { pageFrom } from "../../shared/lib/format";
import { EmptyState, ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
import type { Video, VideoPage as VideoPageData } from "../../types";

export function FavoritesPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 24, total: 0, has_next: false });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const page = pageFrom(searchParams);

  useEffect(() => {
    setLoading(true);
    setError("");
    const controller = new AbortController();
    api.favoriteVideos(new URLSearchParams({ page: String(page), page_size: "24" }), controller.signal)
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

  const removeFavorite = async (video: Video) => {
    try {
      await api.toggleFavorite(video.id);
      setResult((current) => ({ ...current, items: current.items.filter((item) => item.id !== video.id), total: Math.max(0, current.total - 1) }));
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <div className="page favorites-page">
      <section className="following-head favorites-head">
        <div><p className="eyebrow">稍后再看</p><h1>我的收藏</h1><p>按收藏时间整理你想回来继续观看的作品。</p></div>
        <div className="following-rule favorites-rule"><Bookmark size={20} /><span><strong>{result.total} 条收藏</strong>只展示当前仍可访问的公开投稿</span></div>
      </section>
      <section className="content-heading following-title-row"><div><h2>收藏列表</h2><span>按最近收藏时间排序</span></div></section>
      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : result.items.length ? (
        <><section className="video-grid">{result.items.map((video) => <div className="favorite-video" key={video.id}><VideoCard video={video} /><button type="button" className="favorite-remove-button" onClick={() => removeFavorite(video)} title="取消收藏"><Bookmark size={14} fill="currentColor" />取消收藏</button></div>)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : <EmptyState icon={<Bookmark size={28} />} title="还没有收藏" text="在视频页点击收藏，把想看的作品留在这里。" action={<Link to="/popular" className="primary-button"><Compass size={17} />去发现作品</Link>} />}
    </div>
  );
}
