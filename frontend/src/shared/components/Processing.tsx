import { CircleAlert, CircleCheck, Clock3 } from "lucide-react";
import type { Video } from "../../types";

export type ProcessingStageKey = "queued" | "probing" | "transcoding" | "finalizing" | "completed" | "failed" | "processing";

export const processingStageLabels: Record<ProcessingStageKey, string> = {
  queued: "排队中",
  probing: "探测媒体",
  transcoding: "转码中",
  finalizing: "收尾中",
  completed: "已完成",
  failed: "处理失败",
  processing: "处理中"
};

export function processingStage(video: Video): ProcessingStageKey {
  if (video.processing_status === "failed") return "failed";
  if (video.processing_status === "ready") return "completed";
  const stage = (video.processing_stage || "").trim().toLowerCase();
  if (["queued", "queue", "pending", "waiting"].includes(stage)) return "queued";
  if (["probing", "probe", "analyzing", "analysing"].includes(stage)) return "probing";
  if (["transcoding", "transcode", "encoding"].includes(stage)) return "transcoding";
  if (["finalizing", "finalize", "packaging", "finishing"].includes(stage)) return "finalizing";
  if (["completed", "complete", "ready", "done"].includes(stage)) return "completed";
  if (["failed", "error"].includes(stage)) return "failed";
  return video.processing_status === "pending" ? "queued" : "processing";
}

export const processingProgress = (video: Video) =>
  Math.min(100, Math.max(0, Math.round(Number.isFinite(video.processing_progress) ? video.processing_progress : 0)));

export function ProcessingBadge({ video }: { video: Video }) {
  const stage = processingStage(video);
  const icon = stage === "completed"
    ? <CircleCheck size={13} />
    : stage === "failed"
      ? <CircleAlert size={13} />
      : <Clock3 size={13} />;
  return <span className={`processing-badge ${video.processing_status} stage-${stage}`}>{icon}{processingStageLabels[stage]}</span>;
}

export function ProcessingProgress({ video, compact = false }: { video: Video; compact?: boolean }) {
  const progress = processingProgress(video);
  const stage = processingStage(video);
  return (
    <div
      className={`processing-progress ${compact ? "compact-progress" : ""}`}
      role="progressbar"
      aria-label={`媒体${processingStageLabels[stage]}进度`}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={progress}
    >
      <div className="processing-progress-copy"><span>{processingStageLabels[stage]}</span><strong>{progress}%</strong></div>
      <div className="processing-progress-track" aria-hidden="true"><span style={{ width: `${progress}%` }} /></div>
    </div>
  );
}
