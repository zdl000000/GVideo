import { ChevronLeft, ChevronRight } from "lucide-react";

export function Pagination({ page, pageSize, total, hasNext, onPageChange, label = "视频分页" }: { page: number; pageSize: number; total: number; hasNext: boolean; onPageChange: (page: number) => void; label?: string }) {
  if (total <= pageSize && page === 1) return null;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const candidates = Array.from(new Set([1, page - 1, page, page + 1, totalPages])).filter((value) => value >= 1 && value <= totalPages).sort((a, b) => a - b);
  return (
    <nav className="pagination" aria-label={label}>
      <button type="button" className="pagination-arrow" disabled={page <= 1} onClick={() => onPageChange(page - 1)} aria-label="上一页" title="上一页"><ChevronLeft size={18} /></button>
      <div className="pagination-pages">
        {candidates.map((value, index) => <span key={value}>{index > 0 && value - candidates[index - 1] > 1 && <i>...</i>}<button type="button" className={value === page ? "active" : ""} aria-current={value === page ? "page" : undefined} onClick={() => onPageChange(value)}>{value}</button></span>)}
      </div>
      <button type="button" className="pagination-arrow" disabled={!hasNext} onClick={() => onPageChange(page + 1)} aria-label="下一页" title="下一页"><ChevronRight size={18} /></button>
      <span className="pagination-total">共 {totalPages} 页</span>
    </nav>
  );
}
