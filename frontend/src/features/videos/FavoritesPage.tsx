import { useEffect, useRef, useState } from "react";
import { Bookmark, Check, Compass } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { pageFrom } from "../../shared/lib/format";
import { ErrorBlock, LoadingGrid } from "../../shared/components/Feedback";
import { WatchEmptyState } from "../../shared/components/WatchEmptyState";
import { Pagination } from "../../shared/components/Pagination";
import { VideoCard } from "../../shared/components/VideoCard";
import { SectionHeader } from "../../shared/components/DiscoveryControls";
import type { Video, VideoPage as VideoPageData } from "../../types";

export function FavoritesPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 24, total: 0, has_next: false });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  const generation = useRef(0);
  const page = pageFrom(searchParams);

  useEffect(() => {
    const requestID = ++generation.current;
    setLoading(true);
    setError("");
    const controller = new AbortController();
    api.favoriteVideos(new URLSearchParams({ page: String(page), page_size: "24" }), controller.signal)
      .then((data) => {
        if (controller.signal.aborted) return;
        const lastPage = Math.max(1, Math.ceil(data.total / data.page_size));
        if (page > lastPage) {
          const next = new URLSearchParams(searchParams);
          lastPage > 1 ? next.set("page", String(lastPage)) : next.delete("page");
          setSearchParams(next, { replace: true });
          return;
        }
        setResult(data);
      })
      .catch((err) => { if (!controller.signal.aborted) setError(errorMessage(err)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => { controller.abort(); if (generation.current === requestID) generation.current += 1; };
  }, [page, revision, searchParams, setSearchParams]);

  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };
  const renderedGeneration = generation.current;

  return (
    <div className="page gv-discovery-page favorites-page">
      <header className="gv-discovery-header"><div><p className="eyebrow">SAVED / 收藏</p><h1>我的收藏</h1><p>把想回来的作品，留在这里。</p></div><span className="gv-content-count" role="status">{loading ? "正在加载收藏" : error ? "内容暂时无法加载" : `${result.total} 条收藏`}</span></header>
      <SectionHeader title="收藏列表" detail="按最近收藏时间排序" />
      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : result.items.length ? <><section className="video-grid gv-saved-grid" aria-label="收藏视频">{result.items.map((video) => <FavoriteItem key={`${page}-${revision}-${video.id}`} video={video} onRefresh={() => setRevision((value) => value + 1)} onRemoved={() => { if (generation.current === renderedGeneration) setResult((current) => ({ ...current, total: Math.max(0, current.total - 1) })); }} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></> : <WatchEmptyState eyebrow="02 / SAVED" icon={<Bookmark size={28} />} title="还没有收藏" text="在视频页点击收藏，把想看的作品留在这里。" action={<Link to="/popular" className="primary-button"><Compass size={17} />去发现作品</Link>} />}
    </div>
  );
}

function FavoriteItem({ video, onRemoved, onRefresh }: { video: Video; onRemoved: () => void; onRefresh: () => void }) {
  const [pending, setPending] = useState(false);
  const [removed, setRemoved] = useState(false);
  const [error, setError] = useState("");
  const alive = useRef(true);
  const inFlight = useRef(false);
  const statusRef = useRef<HTMLDivElement>(null);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  useEffect(() => { if (removed) statusRef.current?.focus(); }, [removed]);
  const remove = async () => {
    if (inFlight.current || removed) return;
    inFlight.current = true;
    setPending(true);
    setError("");
    try {
      const response = await api.toggleFavorite(video.id);
      if (!alive.current) return;
      if (response.active) { setError("收藏状态已变化，请刷新后重试。"); return; }
      setRemoved(true);
      onRemoved();
    } catch (err) { if (alive.current) setError(errorMessage(err)); }
    finally { inFlight.current = false; if (alive.current) setPending(false); }
  };
  return <div className={`gv-saved-entry ${removed ? "gv-saved-entry--removed" : ""}`}>
    <div inert={removed} aria-hidden={removed ? true : undefined} className="gv-saved-content"><VideoCard video={video} variant="standard" action={<><button type="button" className="favorite-remove-button" disabled={pending || removed} aria-label={`取消收藏 ${video.title}`} onClick={remove}><Bookmark size={15} fill="currentColor" />{pending ? "正在取消…" : "取消收藏"}</button>{error && <p className="gv-save-error" role="alert">{error}</p>}</>} /></div>
    {removed && <div className="gv-saved-removed" role="status" tabIndex={-1} ref={statusRef}><Check size={24} /><strong>已取消收藏</strong><span>其余作品保留原位</span><button type="button" className="secondary-button" onClick={onRefresh}>更新列表</button></div>}
  </div>;
}
