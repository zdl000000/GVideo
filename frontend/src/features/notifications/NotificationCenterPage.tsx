import { useEffect, useState } from "react";
import {
  Bell,
  Bookmark,
  Check,
  CheckCheck,
  CircleAlert,
  CircleCheck,
  Heart,
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
    case "follow": return { title: `${actor} 关注了你`, detail: "有新的观众开始关注你的创作", icon: <Users size={18} /> };
    case "like": return { title: `${actor} 点赞了《${title}》`, detail: "你的作品收到了一个赞", icon: <Heart size={18} /> };
    case "favorite": return { title: `${actor} 收藏了《${title}》`, detail: "你的作品被加入收藏", icon: <Bookmark size={18} /> };
    case "comment": return { title: `${actor} 评论了《${title}》`, detail: item.comment_preview || "查看这条新评论", icon: <MessageCircle size={18} /> };
    case "processing_ready": return { title: `《${title}》已处理完成`, detail: "视频已经可以正常播放", icon: <CircleCheck size={18} /> };
    case "processing_failed": return { title: `《${title}》处理失败`, detail: item.comment_preview || "请检查视频并重新尝试", icon: <CircleAlert size={18} /> };
  }
};

export function NotificationCenterPage() {
  const [params, setParams] = useSearchParams();
  const page = pageFrom(params);
  const [result, setResult] = useState<NotificationPageData>({ items: [], page: 1, page_size: 20, total: 0, has_next: false, unread_count: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<number | "all" | null>(null);

  const load = () => {
    setLoading(true);
    setError("");
    const query = new URLSearchParams({ page: String(page), page_size: "20" });
    api.notifications(query)
      .then(setResult)
      .catch((err) => setError(errorMessage(err)))
      .finally(() => setLoading(false));
  };

  useEffect(load, [page]);

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
    <div className="page notifications-page">
      <section className="page-heading heading-row notification-heading">
        <div><p className="eyebrow">消息动态</p><h1>通知中心</h1><p>{result.unread_count ? `${result.unread_count} 条未读通知` : "所有通知都已读"}</p></div>
        <button className="secondary-button" onClick={markAll} disabled={!result.unread_count || Boolean(busy)}><CheckCheck size={17} />全部标记已读</button>
      </section>
      {error && <p className="inline-error">{error}</p>}
      {loading ? <LoadingBlock label="正在读取通知" /> : result.items.length ? (
        <section className="notification-list" aria-label="通知列表">
          {result.items.map((item) => {
            const copy = notificationCopy(item);
            const target = item.video_id ? `/video/${item.video_id}` : item.actor_id ? `/users/${item.actor_id}` : "/notifications";
            return (
              <article key={item.id} className={`notification-row ${item.read_at ? "read" : "unread"}`}>
                <span className={`notification-icon ${item.type}`}>{copy.icon}</span>
                <div className="notification-copy"><Link to={target} onClick={() => markRead(item)}>{copy.title}</Link><p>{copy.detail}</p><time>{formatDate(item.created_at)}</time></div>
                {!item.read_at && <button className="icon-button" onClick={() => markRead(item)} disabled={Boolean(busy)} title="标记已读" aria-label="标记已读"><Check size={18} /></button>}
              </article>
            );
          })}
        </section>
      ) : <div className="notification-empty"><Bell size={28} /><strong>暂时没有通知</strong><span>新的关注、互动和处理状态会显示在这里</span></div>}
      <Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={(value) => setParams(value > 1 ? { page: String(value) } : {})} />
    </div>
  );
}
