import type { HTMLAttributes } from "react";

export function Skeleton({ variant = "text", className = "", ...props }: HTMLAttributes<HTMLSpanElement> & { variant?: "text" | "media" }) {
  return <span {...props} aria-hidden="true" className={`gv-skeleton gv-skeleton--${variant} ${className}`.trim()} />;
}
