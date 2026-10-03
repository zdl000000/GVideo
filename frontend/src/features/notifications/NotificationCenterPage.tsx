import { useEffect, useRef, useState } from "react";
import {
  Bookmark,
  Check,
  CheckCheck,
  CircleAlert,
  CircleCheck,
  Heart,
  LoaderCircle,
  MessageCircle,
  Users
} from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { LoadingBlock } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { errorMessage } from "../../shared/lib/errors";
import { formatDate, pageFrom } from "../../shared/lib/format";
import type { Notification, NotificationPage as NotificationPageData } from "../../types";

export const notificationCopy = (item: Notification) => {
  const actor = item.actor_username || "有用户";
  const title = item.video_title || "你的视频";
  switch (item.type) {
    case "follow": return { label: "关注", title: `${actor} 关注了你`, detail: "有新的观众开始关注你的创作", icon: <Users size={18} /> };
    case "like": return { label: "点赞", title: `${actor} 点赞了《${title}》`, detail: "你的作品收到了一个赞", icon: <Heart size={18} /> };
    case "favorite": return { label: "收藏", title: `${actor} 收藏了《${title}》`, detail: "你的作品被加入收藏", icon: <Bookmark size={18} /> };
    case "comment": return { label: "评论", title: `${actor} 评论了《${title}》`, detail: item.comment_preview || "查看这条新评论", icon: <MessageCircle size={18} /> };
    case "processing_ready": return { label: "处理完成", title: `《${title}》已处理完成`, detail: "视频已经可以正常播放", icon: <CircleCheck size={18} /> };
    case "processing_failed": return { label: "处理失败", title: `《${title}》处理失败`, detail: item.comment_preview || "请检查视频并重新尝试", icon: <CircleAlert size={18} /> };
  }
};

function NotificationRow({ item, busy, onRead }: { item: Notification; busy: number | "all" | null; onRead: (item: Notification) => Promise<void> }) {
  const linkRef = useRef<HTMLAnchorElement>(null);
  const readButtonRef = useRef<HTMLButtonElement>(null);
  const readFocusRef = useRef(false);
  const copy = notificationCopy(item);
  const target = item.video_id ? `/video/${item.video_id}` : item.actor_id ? `/users/${item.actor_id}` : "/notifications";
  const marking = busy === item.id;
  useEffect(() => {
    if (!readFocusRef.current) return;
    const active = document.activeElement;
    if (active === document.body || active === readButtonRef.current || active === linkRef.current) {
      // The read button is disabled while saving and removed after success.
      // Keep its keyboard context on the same notification, including failures.
      (item.read_at || marking ? linkRef.current : readButtonRef.current)?.focus();
    }
    if (!marking) readFocusRef.current = false;
  }, [item.read_at, marking]);
  return (
    <article className={`gv-notification-row gv-notification-row--${item.type} ${item.read_at ? "read" : "unread"}`} aria-busy={marking}>
      <span className="gv-notification-icon" aria-hidden="true">{copy.icon}</span>
      <div className="gv-notification-copy">
        <div className="gv-notification-meta"><span>{copy.label}</span>{!item.read_at && <span className="gv-notification-unread"><i aria-hidden="true" />未读</span>}<time dateTime={item.created_at}>{formatDate(item.created_at)}</time></div>
        <Link ref={linkRef} to={target} onClick={() => onRead(item)}>{copy.title}</Link>
        <p>{copy.detail}</p>
        {marking && <span className="gv-notification-read-status" role="status">正在标记已读</span>}
      </div>
      {!item.read_at && <button ref={readButtonRef} type="button" className="gv-notification-read-button" onClick={() => { readFocusRef.current = document.activeElement === readButtonRef.current; void onRead(item); }} disabled={busy !== null} title="标记已读" aria-label="标记已读">{marking ? <LoaderCircle size={18} className="gv-notification-busy-icon" aria-hidden="true" /> : <Check size={18} aria-hidden="true" />}</button>}
    </article>
  );
}

export function NotificationCenterPage() {
  const [params, setParams] = useSearchParams();
  const page = pageFrom(params);
  const [result, setResult] = useState<NotificationPageData>({ items: [], page: 1, page_size: 20, total: 0, has_next: false, unread_count: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<number | "all" | null>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const allReadButtonRef = useRef<HTMLButtonElement>(null);
  const allReadFocusRef = useRef(false);

  useEffect(() => {
    if (!allReadFocusRef.current) return;
    const active = document.activeElement;
    if (active === document.body || active === allReadButtonRef.current || active === headingRef.current) {
      (busy === "all" || result.unread_count === 0 ? headingRef.current : allReadButtonRef.current)?.focus();
    }
    if (busy !== "all") allReadFocusRef.current = false;
  }, [busy, result.unread_count]);

  const load = (signal: AbortSignal) => {
    setLoading(true);
    setError("");
    const query = new URLSearchParams({ page: String(page), page_size: "20" });
    api.notifications(query, signal)
      .then((next) => { if (!signal.aborted) setResult(next); })
      .catch((err) => { if (!signal.aborted) setError(errorMessage(err)); })
      .finally(() => { if (!signal.aborted) setLoading(false); });
  };

  useEffect(() => {
    const controller = new AbortController();
    load(controller.signal);
    return () => controller.abort();
  }, [page]);

  const markRead = async (item: Notification) => {
    if (item.read_at || busy) return;
    setBusy(item.id);
    try {
      await api.markNotificationRead(item.id);
      setResult((current) => ({
        ...current,
        unread_count: Math.max(0, current.unread_count - 1),
        items: current.items.map((candidate) => candidate.id === item.id ? { ...candidate, read_at: new Date().toISOString() } : candidate)
      }));
      window.dispatchEvent(new Event("gvideo-notifications-changed"));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  const markAll = async () => {
    if (!result.unread_count || busy) return;
    allReadFocusRef.current = document.activeElement === allReadButtonRef.current;
    setBusy("all");
    try {
      await api.markAllNotificationsRead();
      const readAt = new Date().toISOString();
      setResult((current) => ({ ...current, unread_count: 0, items: current.items.map((item) => ({ ...item, read_at: item.read_at || readAt })) }));
      window.dispatchEvent(new Event("gvideo-notifications-changed"));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="page gv-notification-center">
      <section className="gv-notification-heading" aria-labelledby="notification-center-title">
        <div><p className="eyebrow">WATCH / ACTIVITY</p><h1 ref={headingRef} tabIndex={-1} id="notification-center-title">通知中心</h1><p className="gv-notification-count" aria-live="polite">{loading ? "正在读取通知动态" : result.unread_count ? `${result.unread_count} 条未读通知` : "所有通知都已读"}</p></div>
        <button ref={allReadButtonRef} type="button" className="secondary-button" onClick={markAll} disabled={loading || !result.unread_count || busy !== null}>{busy === "all" ? <LoaderCircle size={17} className="gv-notification-busy-icon" aria-hidden="true" /> : <CheckCheck size={17} aria-hidden="true" />}{busy === "all" ? "正在标记全部已读" : "全部标记已读"}</button>
      </section>
      {error && <p className="inline-error" role="alert">{error}</p>}
      {loading ? <LoadingBlock label="正在读取通知" /> : result.items.length ? (
        <section className="gv-notification-feed" aria-label="通知列表">
          {result.items.map((item) => <NotificationRow key={item.id} item={item} busy={busy} onRead={markRead} />)}
        </section>
      ) : !error && <section className="gv-notification-empty" aria-labelledby="notification-empty-title"><p className="eyebrow">ACTIVITY / 00</p><h2 id="notification-empty-title">暂时没有通知</h2><p>新的关注、互动和处理状态会显示在这里。</p><span className="gv-notification-empty-motif" aria-hidden="true"><i /><i /><i /></span></section>}
      <Pagination label="通知分页" page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={(value) => setParams(value > 1 ? { page: String(value) } : {})} />
    </div>
  );
}
