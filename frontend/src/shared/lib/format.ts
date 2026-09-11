export const formatCount = (value: number) =>
  new Intl.NumberFormat("zh-CN", { notation: value >= 10000 ? "compact" : "standard", maximumFractionDigits: 1 }).format(value);

export const formatDuration = (seconds: number) => {
  if (!seconds) return "--:--";
  const minutes = Math.floor(seconds / 60);
  const rest = Math.floor(seconds % 60);
  return `${minutes}:${rest.toString().padStart(2, "0")}`;
};

export const formatDate = (value: string) =>
  new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(new Date(value));

export const formatJoinDate = (value: string) =>
  new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "long", day: "numeric" }).format(new Date(value));

export const formatFileSize = (bytes: number) => bytes >= 1024 * 1024
  ? `${(bytes / 1024 / 1024).toFixed(1)} MB`
  : `${Math.max(1, Math.round(bytes / 1024))} KB`;

export const pageFrom = (params: URLSearchParams) => Math.max(1, Number.parseInt(params.get("page") || "1", 10) || 1);
