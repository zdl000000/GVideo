import { useId, type ReactNode } from "react";

export function WatchEmptyState({ eyebrow, icon, title, text, action }: { eyebrow: string; icon: ReactNode; title: string; text: string; action: ReactNode }) {
  const titleID = useId();
  return <section className="gv-watch-empty" aria-labelledby={titleID}>
    <div className="gv-watch-empty-copy"><p className="gv-watch-empty-eyebrow">{eyebrow}</p><h2 id={titleID}>{title}</h2><p>{text}</p>{action}</div>
    <div className="gv-watch-empty-art" aria-hidden="true"><span />{icon}<span>GVIDEO / WATCH</span></div>
  </section>;
}
