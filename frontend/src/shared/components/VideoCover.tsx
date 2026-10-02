import { useState } from "react";
import { Play } from "lucide-react";
import mark from "../../assets/gvideo-mark.svg";

/** Remount the image state when its URL changes; a failed image is never retried in a loop. */
export function VideoCover({ src, eager = false }: { src: string; eager?: boolean }) {
  return <CoverImage key={src} src={src} eager={eager} />;
}

function CoverImage({ src, eager }: { src: string; eager: boolean }) {
  const [state, setState] = useState<"loading" | "ready" | "failed">(src ? "loading" : "failed");
  return (
    <span className={`gv-video-cover gv-video-cover--${state}`} aria-hidden="true">
      {state !== "ready" && <span className="gv-media-fallback"><span className="gv-media-frame" /><img src={mark} alt="" /><span className="gv-media-label">GVIDEO / MEDIA</span><span className="gv-media-caption">封面占位</span><Play size={16} /></span>}
      {src && state !== "failed" && <img className="gv-cover-image" src={src} alt="" loading={eager ? "eager" : "lazy"} fetchPriority={eager ? "high" : "auto"} decoding="async" onLoad={() => setState("ready")} onError={() => setState("failed")} />}
    </span>
  );
}
