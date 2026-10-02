import type { ReactNode } from "react";
import { ArrowUpRight, Clock3, Flame } from "lucide-react";
import { Link } from "react-router-dom";

export function CategoryFilter({ categories, value, onChange }: { categories: string[]; value: string; onChange: (value: string) => void }) {
  const options = Array.from(new Set(["", ...categories, ...(value ? [value] : [])]));
  return <div className="category-scroller" role="group" aria-label="视频分类">{options.map((category) => <button type="button" key={category} className={value === category ? "active" : ""} aria-pressed={value === category} onClick={() => onChange(category)}>{category || "全部"}</button>)}</div>;
}

export function SortSwitch({ params, active }: { params: URLSearchParams; active: "latest" | "popular" }) {
  const next = new URLSearchParams(params);
  next.delete("page");
  next.delete("sort");
  const query = next.size ? `?${next}` : "";
  return <nav className="segmented-control" aria-label="视频排序"><Link className={active === "latest" ? "active" : ""} aria-current={active === "latest" ? "page" : undefined} to={`/latest${query}`}><Clock3 size={15} />最新</Link><Link className={active === "popular" ? "active" : ""} aria-current={active === "popular" ? "page" : undefined} to={`/popular${query}`}><Flame size={15} />热门</Link></nav>;
}

export function SectionHeader({ title, detail, to, linkLabel = "查看全部" }: { title: string; detail?: ReactNode; to?: string; linkLabel?: string }) {
  return <header className="gv-section-header"><div><h2>{title}</h2>{detail != null && <span>{detail}</span>}</div>{to && <Link to={to}>{linkLabel}<ArrowUpRight size={16} /></Link>}</header>;
}
