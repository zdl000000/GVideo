import { UserRound, X } from "lucide-react";
import { Link } from "react-router-dom";
import { Button } from "../ui/Button";
import { Skeleton } from "../ui/Skeleton";

export { EmptyState } from "../ui/EmptyState";

export function LoadingGrid() { return <div className="video-grid" role="status" aria-label="正在加载视频">{Array.from({ length: 8 }, (_, index) => <div className="skeleton-card" key={index}><Skeleton variant="media" /><Skeleton /><Skeleton className="gv-skeleton--short" /></div>)}</div>; }

export function LoadingBlock({ label }: { label: string }) { return <div className="state-block" role="status"><span className="spinner" aria-hidden="true" /><p>{label}</p></div>; }

export function ErrorBlock({ message }: { message: string }) { return <div className="state-block error-state" role="alert"><X size={28} aria-hidden="true" /><h2>内容加载失败</h2><p>{message}</p><Button variant="secondary" onClick={() => window.location.reload()}>重新加载</Button></div>; }

export function NotFound() { return <div className="state-block"><UserRound size={30} /><h1>页面没有找到</h1><p>这个地址可能已经改变。</p><Link className="primary-button" to="/">返回首页</Link></div>; }
