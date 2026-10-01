import type { ReactNode } from "react";

export function EmptyState({ icon, title, text, action }: { icon?: ReactNode; title: string; text: string; action?: ReactNode }) {
  return <div className="state-block empty-state"><span aria-hidden="true">{icon}</span><h2>{title}</h2><p>{text}</p>{action}</div>;
}
