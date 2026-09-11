import { useEffect, useRef, useState } from "react";
import { Check, Clock3, Compass, Film, LayoutDashboard, Pencil, Upload, UserRound } from "lucide-react";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { formatCount, formatJoinDate, pageFrom } from "../../shared/lib/format";
import { Avatar } from "../../shared/components/Avatar";
import { VideoCard } from "../../shared/components/VideoCard";
import { Pagination } from "../../shared/components/Pagination";
import { LoadingBlock, ErrorBlock, EmptyState } from "../../shared/components/Feedback";
import type { CreatorProfile, User, VideoPage as VideoPageData } from "../../types";
import { EditProfileDialog } from "./EditProfileDialog";

export function CreatorPage({ user, onUserUpdated }: { user: User | null; onUserUpdated: (user: User) => void }) {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [profile, setProfile] = useState<CreatorProfile | null>(null);
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 12, total: 0, has_next: false });
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [editingProfile, setEditingProfile] = useState(false);
  const editProfileButtonRef = useRef<HTMLButtonElement>(null);
  const page = pageFrom(searchParams);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    Promise.all([
      api.creator(id, controller.signal),
      api.creatorVideos(id, new URLSearchParams({ page: String(page), page_size: "12" }), controller.signal)
    ]).then(([nextProfile, nextVideos]) => {
      if (controller.signal.aborted) return;
      setProfile(nextProfile);
      setResult(nextVideos);
    }).catch((err) => { if (!controller.signal.aborted) setError(errorMessage(err)); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [id, page]);

  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const toggleFollow = async () => {
    if (!profile) return;
    if (!user) {
      navigate(`/auth?next=${encodeURIComponent(`${location.pathname}${location.search}`)}`);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const next = await api.toggleFollow(profile.id);
      setProfile((current) => current ? {
        ...current,
        followed: next.active,
        followers_count: Math.max(0, current.followers_count + (next.active ? 1 : -1))
      } : current);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  if (loading) return <LoadingBlock label="正在打开作者空间" />;
  if (error && !profile) return <ErrorBlock message={error} />;
  if (!profile) return null;
  const isSelf = user?.id === profile.id;

  return (
    <div className="page creator-page">
      <section className="creator-hero" aria-labelledby="creator-name">
        <Avatar username={profile.username} src={profile.avatar_url} size="large" className="creator-hero-avatar" />
        <div className="creator-identity">
          <p className="eyebrow">作者空间</p>
          <h1 id="creator-name">{profile.username}</h1>
          <p>{profile.bio || "这位创作者还没有填写个人简介。"}</p>
          <span className="creator-joined"><Clock3 size={14} />{formatJoinDate(profile.created_at)} 加入</span>
        </div>
        <dl className="creator-stats">
          <div><dt>投稿</dt><dd>{formatCount(profile.videos_count)}</dd></div>
          <div><dt>粉丝</dt><dd>{formatCount(profile.followers_count)}</dd></div>
          <div><dt>关注</dt><dd>{formatCount(profile.following_count)}</dd></div>
        </dl>
        <div className="creator-action">
          {isSelf ? (
            <>
              <button ref={editProfileButtonRef} type="button" className="secondary-button" onClick={() => setEditingProfile(true)}><Pencil size={17} />编辑资料</button>
              <Link to="/creator" className="secondary-button"><LayoutDashboard size={17} />创作者中心</Link>
              <Link to="/me/videos" className="primary-button"><Film size={17} />管理投稿</Link>
            </>
          ) : (
            <button type="button" className={profile.followed ? "secondary-button follow-button active" : "primary-button follow-button"} disabled={busy} onClick={toggleFollow} aria-pressed={profile.followed}>
              {profile.followed ? <Check size={17} /> : <UserRound size={17} />}{busy ? "处理中..." : profile.followed ? "已关注" : "关注"}
            </button>
          )}
        </div>
      </section>
      {error && <p className="inline-error creator-error">{error}</p>}
      <section className="creator-content-head"><div><p className="eyebrow">全部投稿</p><h2>{profile.username} 的作品</h2></div><span>共 {result.total} 条</span></section>
      {result.items.length ? (
        <><section className="video-grid">{result.items.map((video) => <VideoCard video={video} key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Film size={28} />} title="还没有公开投稿" text={isSelf ? "发布第一条作品，让个人空间丰富起来。" : "这位创作者发布作品后会显示在这里。"} action={isSelf ? <Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link> : <Link to="/popular" className="secondary-button"><Compass size={17} />浏览热门</Link>} />
      )}
      {editingProfile && (
        <EditProfileDialog
          profile={profile}
          onClose={() => { setEditingProfile(false); window.setTimeout(() => editProfileButtonRef.current?.focus(), 0); }}
          onSaved={(updated) => {
            setProfile((current) => current ? { ...current, ...updated } : current);
            onUserUpdated(updated);
            setEditingProfile(false);
            window.setTimeout(() => editProfileButtonRef.current?.focus(), 0);
          }}
        />
      )}
    </div>
  );
}
