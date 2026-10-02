import { useEffect, useState } from "react";
import { Check, CircleCheck, CircleX, Clock3, Flag, ScanEye, X } from "lucide-react";
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

const reportStatusIcons = { pending: Clock3, reviewed: ScanEye, resolved: CircleCheck, dismissed: CircleX };

function ReportRow({ report, busy, onReview }: { report: VideoReport; busy: number | null; onReview: (report: VideoReport, status: VideoReportStatus) => void }) {
  const StatusIcon = reportStatusIcons[report.status];
  return (
    <article className="gv-report-row" aria-labelledby={`report-title-${report.id}`} aria-busy={busy === report.id} data-status={report.status}>
      <div className="gv-report-state">
        <span className={`gv-report-status gv-report-status--${report.status}`}><StatusIcon size={14} aria-hidden="true" />{reportStatusLabels[report.status]}</span>
        <span className="gv-report-id">REPORT / #{report.id}</span>
      </div>
      <div className="gv-report-content">
        <h2 id={`report-title-${report.id}`}><Link to={`/video/${report.video_id}`}>{report.video_title || `视频 #${report.video_id}`}</Link></h2>
        <p className="gv-report-reason"><Flag size={13} aria-hidden="true" /><span>{reportReasonLabels[report.reason] || report.reason}</span></p>
        <p className={`gv-report-detail${report.detail ? "" : " gv-report-detail--empty"}`}>{report.detail || "举报人未填写补充说明"}</p>
        <div className="gv-report-people">
          <span>举报人 <Link to={`/users/${report.user_id}`}>{report.reporter_username || `用户 #${report.user_id}`}</Link></span>
          <span>作者 <Link to={`/users/${report.video_author_id}`}>{report.video_author || `用户 #${report.video_author_id}`}</Link></span>
        </div>
        <div className="gv-report-dates">
          <span>提交 <time dateTime={report.created_at}>{formatDate(report.created_at)}</time></span>
          <span>更新 <time dateTime={report.updated_at}>{formatDate(report.updated_at)}</time></span>
        </div>
      </div>
      <div className="gv-report-actions" role="group" aria-label={`处理举报 #${report.id}`}>
        <button type="button" disabled={Boolean(busy) || report.status === "reviewed"} onClick={() => onReview(report, "reviewed")}><Clock3 size={15} aria-hidden="true" />审核中</button>
        <button type="button" disabled={Boolean(busy) || report.status === "resolved"} onClick={() => onReview(report, "resolved")}><Check size={15} aria-hidden="true" />处理完成</button>
        <button type="button" className="gv-report-dismiss" disabled={Boolean(busy) || report.status === "dismissed"} onClick={() => onReview(report, "dismissed")}><X size={15} aria-hidden="true" />驳回</button>
        {busy === report.id && <span className="gv-report-busy" role="status">正在更新举报状态…</span>}
      </div>
    </article>
  );
}

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
    <div className="page admin-reports-page gv-governance-page">
      <header className="page-heading gv-governance-heading">
        <div><p className="eyebrow">GOVERNANCE / REPORTS</p><h1>举报审核</h1><p>内容治理 · 优先处理待审核内容</p></div>
        <div className="gv-governance-count"><strong>{result.total.toLocaleString("zh-CN")}</strong><span>举报记录</span></div>
      </header>
      <div className="gv-governance-filter-bar">
        <div className="gv-governance-filters" role="group" aria-label="举报状态筛选">
            {reportFilters.map((filter) => (
              <button key={filter.value || "all"} type="button" className={status === filter.value ? "active" : ""} onClick={() => updateQuery(filter.value)} aria-pressed={status === filter.value}>{filter.label}</button>
            ))}
        </div>
        <span className="gv-governance-filter-note">{status ? reportStatusLabels[status] : "全部状态"}</span>
      </div>
      {error && <p className="inline-error admin-report-error" role="alert">{error}</p>}
      {loading ? <LoadingBlock label="正在读取举报记录" /> : result.items.length ? (
        <section className="gv-governance-records" aria-label="举报记录">
          <div className="gv-governance-columns" aria-hidden="true"><span>状态 / 编号</span><span>举报内容</span><span>审核操作</span></div>
          {result.items.map((report) => (
            <ReportRow key={report.id} report={report} busy={busy} onReview={review} />
          ))}
        </section>
      ) : <section className="gv-governance-empty"><p className="eyebrow">REPORTS / 00</p><Flag size={26} aria-hidden="true" /><h2>当前筛选下没有举报</h2><p>新的举报提交后会显示在这里</p></section>}
      <Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} label="举报记录分页" onPageChange={(value) => updateQuery(status, value)} />
    </div>
  );
}
