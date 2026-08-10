import type { AuthPayload, Comment, CreatorProfile, SubtitleTrack, User, Video, VideoPage } from "./types";

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

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (!(init.body instanceof FormData) && init.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }
  if (csrfToken && init.method && init.method !== "GET") {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const response = await fetch(path, { ...init, headers, credentials: "include" });
  const payload = (await response.json().catch(() => ({}))) as Envelope<T>;
  if (!response.ok) {
    throw new ApiError(payload.error || "请求失败，请稍后重试", response.status, payload.request_id);
  }
  return payload.data as T;
}

export const api = {
  categories: () => request<string[]>("/api/v1/categories"),
  me: () => request<AuthPayload>("/api/v1/auth/me"),
  register: (username: string, password: string) =>
    request<AuthPayload>("/api/v1/auth/register", { method: "POST", body: JSON.stringify({ username, password }) }),
  login: (username: string, password: string) =>
    request<AuthPayload>("/api/v1/auth/login", { method: "POST", body: JSON.stringify({ username, password }) }),
  logout: () => request<{ logged_out: boolean }>("/api/v1/auth/logout", { method: "POST" }),
  videos: (params: URLSearchParams) => request<VideoPage>(`/api/v1/videos?${params.toString()}`),
  myVideos: (params = new URLSearchParams()) => request<VideoPage>(`/api/v1/me/videos?${params.toString()}`),
  followingVideos: (params = new URLSearchParams()) => request<VideoPage>(`/api/v1/me/following/videos?${params.toString()}`),
  creator: (id: string) => request<CreatorProfile>(`/api/v1/users/${encodeURIComponent(id)}`),
  creatorVideos: (id: string, params = new URLSearchParams()) => request<VideoPage>(`/api/v1/users/${encodeURIComponent(id)}/videos?${params.toString()}`),
  toggleFollow: (id: number) => request<{ active: boolean }>(`/api/v1/users/${id}/follow`, { method: "POST" }),
  video: (id: string, countView = true) => request<Video>(`/api/v1/videos/${id}?count_view=${countView}`),
  upload: (form: FormData) => request<Video>("/api/v1/videos", { method: "POST", body: form }),
  updateVideo: (id: number, form: FormData) => request<Video>(`/api/v1/videos/${id}`, { method: "PATCH", body: form }),
  deleteVideo: (id: number) => request<{ deleted: boolean }>(`/api/v1/videos/${id}`, { method: "DELETE" }),
  retryVideo: (id: number) => request<{ processing_status: string }>(`/api/v1/videos/${id}/retry`, { method: "POST" }),
  uploadSubtitle: (id: number, form: FormData) => request<SubtitleTrack>(`/api/v1/videos/${id}/subtitles`, { method: "POST", body: form }),
  toggleLike: (id: number) => request<{ active: boolean }>(`/api/v1/videos/${id}/like`, { method: "POST" }),
  toggleFavorite: (id: number) => request<{ active: boolean }>(`/api/v1/videos/${id}/favorite`, { method: "POST" }),
  comments: (id: string) => request<Comment[]>(`/api/v1/videos/${id}/comments`),
  comment: (id: string, content: string) =>
    request<Comment>(`/api/v1/videos/${id}/comments`, { method: "POST", body: JSON.stringify({ content }) })
};

export type { User };
