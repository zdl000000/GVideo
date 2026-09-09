import { useEffect, useState } from "react";

export function Avatar({ username, src, size = "medium", className = "" }: { username: string; src?: string; size?: "small" | "medium" | "large"; className?: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [src]);
  return (
    <span className={`avatar avatar-${size} ${className}`.trim()} aria-hidden="true">
      {src && !failed ? <img src={src} alt="" onError={() => setFailed(true)} /> : username.slice(0, 1).toUpperCase()}
    </span>
  );
}
