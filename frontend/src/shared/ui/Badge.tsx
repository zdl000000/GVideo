import type { HTMLAttributes } from "react";

export function Badge({ tone = "neutral", className = "", ...props }: HTMLAttributes<HTMLSpanElement> & { tone?: "neutral" | "brand" | "success" | "warning" | "danger" }) {
  return <span {...props} className={`gv-badge gv-badge--${tone} ${className}`.trim()} />;
}
