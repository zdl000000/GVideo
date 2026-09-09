import { useEffect, useState } from "react";
import { Check, Clock3, Flag, X } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { LoadingBlock } from "../../shared/components/Feedback";
import { Pagination } from "../../shared/components/Pagination";
import { errorMessage } from "../../shared/lib/errors";
import { formatDate, pageFrom } from "../../shared/lib/format";
import type { VideoReport, VideoReportPage, VideoReportStatus } from "../../types";

export const reportStatusLabels: Record<VideoReportStatus, string> = {
  pending: "待处理",
  reviewed: "审核中",
  resolved: "已处理",
  dismissed: "已驳回"
};

export const reportReasonLabels: Record<string, string> = {
  spam: "垃圾信息",
  inappropriate: "不当内容",
  copyright: "版权问题",
  other: "其他"
};

export const reportFilters: Array<{ value: "" | VideoReportStatus; label: string }> = [
  { value: "", label: "全部" },
  { value: "pending", label: "待处理" },
  { value: "reviewed", label: "审核中" },
  { value: "resolved", label: "已处理" },
  { value: "dismissed", label: "已驳回" }
];

export const isReportStatus = (value: string | null): value is VideoReportStatus =>
  value === "pending" || value === "reviewed" || value === "resolved" || value === "dismissed";

export function AdminReportsPage() {
  const [params, setParams] = useSearchParams();
  const page = pageFrom(params);
  const status = isReportStatus(params.get("status")) ? params.get("status") as VideoReportStatus : "";
  const [result, setResult] = useState<VideoReportPage>({ items: [], page: 1, page_size: 20, total: 0, has_next: false });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<number | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    const query = new URLSearchParams({ page: String(page), page_size: "20" });
    if (status) query.set("status", status);
    setLoading(true);
    setError("");
    api.adminReports(query, controller.signal)
      .then(setResult)
      .catch((err) => {
        if (!(err instanceof DOMException && err.name === "AbortError")) setError(errorMessage(err));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [page, status]);

  const updateQuery = (nextStatus: "" | VideoReportStatus, nextPage = 1) => {
    const query = new URLSearchParams();
    if (nextStatus) query.set("status", nextStatus);
    if (nextPage > 1) query.set("page", String(nextPage));
    setParams(query);
  };

  const review = async (report: VideoReport, nextStatus: VideoReportStatus) => {
    if (busy || report.status === nextStatus) return;
    setBusy(report.id);
    setError("");
    try {
      const updated = await api.reviewReport(report.id, nextStatus);
      setResult((current) => ({
        ...current,
        items: current.items.map((item) => item.id === report.id ? { ...item, ...updated } : item)
      }));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="page admin-reports-page">
      <section className="page-heading admin-report-heading">
        <div><p className="eyebrow">内容治理</p><h1>举报审核</h1><p>共 {result.total} 条记录，优先处理待审核内容</p></div>
        <div className="admin-report-toolbar" aria-label="举报状态筛选">
          <div className="segmented-control">
            {reportFilters.map((filter) => (
              <button key={filter.value || "all"} type="button" className={status === filter.value ? "active" : ""} onClick={() => updateQuery(filter.value)} aria-pressed={status === filter.value}>{filter.label}</button>
            ))}
          </div>
        </div>
      </section>
      {error && <p className="inline-error admin-report-error">{error}</p>}
      {loading ? <LoadingBlock label="正在读取举报记录" /> : result.items.length ? (
        <section className="admin-report-list" aria-label="举报记录">
          {result.items.map((report) => (
            <article key={report.id} className="admin-report-row">
              <div className="admin-report-status-column">
                <span className={`admin-report-status ${report.status}`}>{reportStatusLabels[report.status]}</span>
                <span>#{report.id}</span>
              </div>
              <div className="admin-report-main">
                <div className="admin-report-title">
                  <strong>{reportReasonLabels[report.reason] || report.reason}</strong>
                  <Link to={`/video/${report.video_id}`}>{report.video_title || `视频 #${report.video_id}`}</Link>
                </div>
                <p className={`admin-report-detail ${report.detail ? "" : "empty"}`}>{report.detail || "举报人未填写补充说明"}</p>
                <div className="admin-report-meta">
                  <span>作者 <Link to={`/users/${report.video_author_id}`}>{report.video_author || `用户 #${report.video_author_id}`}</Link></span>
                  <span>举报人 <Link to={`/users/${report.user_id}`}>{report.reporter_username || `用户 #${report.user_id}`}</Link></span>
                  <span>提交 {formatDate(report.created_at)}</span>
                  <span>更新 {formatDate(report.updated_at)}</span>
                </div>
              </div>
              <div className="admin-report-actions" aria-label={`处理举报 #${report.id}`}>
                <button type="button" disabled={Boolean(busy) || report.status === "reviewed"} onClick={() => review(report, "reviewed")}><Clock3 size={15} />审核中</button>
                <button type="button" disabled={Boolean(busy) || report.status === "resolved"} onClick={() => review(report, "resolved")}><Check size={15} />处理完成</button>
                <button type="button" className="dismiss" disabled={Boolean(busy) || report.status === "dismissed"} onClick={() => review(report, "dismissed")}><X size={15} />驳回</button>
              </div>
            </article>
          ))}
        </section>
      ) : <div className="admin-report-empty"><Flag size={28} /><strong>当前筛选下没有举报</strong><span>新的举报提交后会显示在这里</span></div>}
      <Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} label="举报记录分页" onPageChange={(value) => updateQuery(status, value)} />
    </div>
  );
}
