import { UserRound, X } from "lucide-react";
import { Link } from "react-router-dom";

export function LoadingGrid() { return <div className="video-grid">{Array.from({ length: 8 }, (_, index) => <div className="skeleton-card" key={index}><span /><i /><i /></div>)}</div>; }

export function LoadingBlock({ label }: { label: string }) { return <div className="state-block"><span className="spinner" /><p>{label}</p></div>; }

export function ErrorBlock({ message }: { message: string }) { return <div className="state-block error-state"><X size={28} /><h2>内容加载失败</h2><p>{message}</p><button className="secondary-button" onClick={() => window.location.reload()}>重新加载</button></div>; }

export function EmptyState({ icon, title, text, action }: { icon: React.ReactNode; title: string; text: string; action: React.ReactNode }) { return <div className="state-block empty-state">{icon}<h2>{title}</h2><p>{text}</p>{action}</div>; }

export function NotFound() { return <div className="state-block"><UserRound size={30} /><h1>页面没有找到</h1><p>这个地址可能已经改变。</p><Link className="primary-button" to="/">返回首页</Link></div>; }
