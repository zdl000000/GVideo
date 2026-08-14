import type { AuthPayload, Comment, CreatorProfile, CreatorStats, NotificationPage, SubtitleTrack, User, Video, VideoPage, VideoReport, VideoReportPage, VideoReportStatus } from "../../types";

interface Envelope<T> {
  data?: T;
  error?: string;
  request_id: string;
}

export class ApiError extends Error {
  status: number;
  requestId: string;

  constructor(message: string, status: number, requestId = "") {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.requestId = requestId;
  }
}

let csrfToken = "";

export function setCSRFToken(token: string) {
  csrfToken = token;
}

async function request<T>(path: string, init: RequestInit = {}, signal?: AbortSignal): Promise<T> {
  const headers = new Headers(init.headers);
  if (!(init.body instanceof FormData) && init.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }
  if (csrfToken && init.method && init.method !== "GET") {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const response = await fetch(path, { ...init, headers, credentials: "include", signal });
  const payload = (await response.json().catch(() => ({}))) as Envelope<T>;
  if (!response.ok) {
    if (path !== "/api/v1/auth/me" && (response.status === 401 || (response.status === 403 && payload.error?.includes("凭证")))) {
      window.dispatchEvent(new CustomEvent("gvideo-auth-expired"));
    }
    throw new ApiError(payload.error || "请求失败，请稍后重试", response.status, payload.request_id);
  }
  return payload.data as T;
}

export function uploadWithProgress<T>(path: string, form: FormData, onProgress: (loaded: number, total: number) => void, signal?: AbortSignal): Promise<T> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    let settled = false;
    const abort = () => {
      xhr.abort();
      if (!settled) {
        settled = true;
        reject(new DOMException("The upload was aborted", "AbortError"));
      }
    };
    if (signal?.aborted) {
      abort();
      return;
    }
    signal?.addEventListener("abort", abort, { once: true });
    const cleanup = () => signal?.removeEventListener("abort", abort);
    xhr.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable) onProgress(event.loaded, event.total);
    });
    xhr.addEventListener("load", () => {
      cleanup();
      if (settled) return;
      let payload: Envelope<T> = {} as Envelope<T>;
      try {
        payload = JSON.parse(xhr.responseText || "{}") as Envelope<T>;
      } catch {
        settled = true;
        reject(new ApiError("服务器返回了无法识别的响应，请稍后重试", xhr.status || 502));
        return;
      }
      settled = true;
      if (xhr.status === 401 || (xhr.status === 403 && payload.error?.includes("凭证"))) window.dispatchEvent(new CustomEvent("gvideo-auth-expired"));
      if (xhr.status < 200 || xhr.status >= 300) reject(new ApiError(payload.error || "请求失败，请稍后重试", xhr.status, payload.request_id));
      else resolve(payload.data as T);
    });
    xhr.addEventListener("error", () => { cleanup(); if (!settled) { settled = true; reject(new Error("网络连接失败，请稍后重试")); } });
    xhr.addEventListener("abort", () => { cleanup(); if (!settled) { settled = true; reject(new DOMException("The upload was aborted", "AbortError")); } });
    xhr.open("POST", path);
    xhr.withCredentials = true;
    if (csrfToken) xhr.setRequestHeader("X-CSRF-Token", csrfToken);
    xhr.send(form);
  });
}

export const api = {
  categories: () => request<string[]>("/api/v1/categories"),
  me: () => request<AuthPayload>("/api/v1/auth/me"),
  register: (username: string, password: string) =>
    request<AuthPayload>("/api/v1/auth/register", { method: "POST", body: JSON.stringify({ username, password }) }),
  login: (username: string, password: string) =>
    request<AuthPayload>("/api/v1/auth/login", { method: "POST", body: JSON.stringify({ username, password }) }),
  logout: () => request<{ logged_out: boolean }>("/api/v1/auth/logout", { method: "POST" }),
  updateProfile: (form: FormData) => request<User>("/api/v1/me/profile", { method: "PATCH", body: form }),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<{ changed: boolean }>("/api/v1/me/password", { method: "POST", body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) }),
  creatorStats: () => request<CreatorStats>("/api/v1/me/creator/stats"),
  notifications: (params = new URLSearchParams(), signal?: AbortSignal) => request<NotificationPage>(`/api/v1/me/notifications?${params.toString()}`, {}, signal),
  markNotificationRead: (id: number) => request<{ read: boolean }>(`/api/v1/me/notifications/${id}/read`, { method: "PATCH" }),
  markAllNotificationsRead: () => request<{ read: boolean }>("/api/v1/me/notifications/read-all", { method: "POST" }),
  videos: (params: URLSearchParams, signal?: AbortSignal) => request<VideoPage>(`/api/v1/videos?${params.toString()}`, {}, signal),
  myVideos: (params = new URLSearchParams(), signal?: AbortSignal) => request<VideoPage>(`/api/v1/me/videos?${params.toString()}`, {}, signal),
  followingVideos: (params = new URLSearchParams(), signal?: AbortSignal) => request<VideoPage>(`/api/v1/me/following/videos?${params.toString()}`, {}, signal),
  favoriteVideos: (params = new URLSearchParams(), signal?: AbortSignal) => request<VideoPage>(`/api/v1/me/favorites?${params.toString()}`, {}, signal),
  creator: (id: string, signal?: AbortSignal) => request<CreatorProfile>(`/api/v1/users/${encodeURIComponent(id)}`, {}, signal),
  creatorVideos: (id: string, params = new URLSearchParams(), signal?: AbortSignal) => request<VideoPage>(`/api/v1/users/${encodeURIComponent(id)}/videos?${params.toString()}`, {}, signal),
  toggleFollow: (id: number) => request<{ active: boolean }>(`/api/v1/users/${id}/follow`, { method: "POST" }),
  video: (id: string, countView = true, signal?: AbortSignal) => request<Video>(`/api/v1/videos/${id}?count_view=${countView}`, {}, signal),
  upload: (form: FormData, onProgress?: (loaded: number, total: number) => void, signal?: AbortSignal) =>
    onProgress ? uploadWithProgress<Video>("/api/v1/videos", form, onProgress, signal) : request<Video>("/api/v1/videos", { method: "POST", body: form }, signal),
  updateVideo: (id: number, form: FormData) => request<Video>(`/api/v1/videos/${id}`, { method: "PATCH", body: form }),
  deleteVideo: (id: number) => request<{ deleted: boolean }>(`/api/v1/videos/${id}`, { method: "DELETE" }),
  retryVideo: (id: number) => request<{ processing_status: string }>(`/api/v1/videos/${id}/retry`, { method: "POST" }),
  uploadSubtitle: (id: number, form: FormData) => request<SubtitleTrack>(`/api/v1/videos/${id}/subtitles`, { method: "POST", body: form }),
  setDefaultSubtitle: (id: number, subtitleID: number) =>
    request<SubtitleTrack[]>(`/api/v1/videos/${id}/subtitles/${subtitleID}/default`, { method: "PATCH" }),
  deleteSubtitle: (id: number, subtitleID: number) =>
    request<SubtitleTrack[]>(`/api/v1/videos/${id}/subtitles/${subtitleID}`, { method: "DELETE" }),
  toggleLike: (id: number) => request<{ active: boolean }>(`/api/v1/videos/${id}/like`, { method: "POST" }),
  toggleFavorite: (id: number) => request<{ active: boolean }>(`/api/v1/videos/${id}/favorite`, { method: "POST" }),
  reportVideo: (id: number, reason: string, detail: string) =>
    request<VideoReport>(`/api/v1/videos/${id}/reports`, { method: "POST", body: JSON.stringify({ reason, detail }) }),
  adminReports: (params = new URLSearchParams(), signal?: AbortSignal) =>
    request<VideoReportPage>(`/api/v1/admin/reports?${params.toString()}`, {}, signal),
  reviewReport: (id: number, status: VideoReportStatus) =>
    request<VideoReport>(`/api/v1/admin/reports/${id}`, { method: "PATCH", body: JSON.stringify({ status }) }),
  comments: (id: string, signal?: AbortSignal) => request<Comment[]>(`/api/v1/videos/${id}/comments`, {}, signal),
  comment: (id: string, content: string) =>
    request<Comment>(`/api/v1/videos/${id}/comments`, { method: "POST", body: JSON.stringify({ content }) }),
  deleteComment: (videoID: number, commentID: number) =>
    request<{ deleted: boolean }>(`/api/v1/videos/${videoID}/comments/${commentID}`, { method: "DELETE" })
};

export type { User };
