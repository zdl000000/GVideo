import type { Video, VideoPage } from "../../types";

// Deterministic test data only; product pages always use the existing API.
export function videoFixture(id = 1, overrides: Partial<Video> = {}): Video {
  return { id, user_id: id + 10, username: `创作者${id}`, avatar_url: "", title: `作品${id}`, description: "",
    category: "音乐", visibility: "public", video_url: "", hls_url: "", cover_url: "", mime_type: "video/mp4",
    duration_seconds: 61, size_bytes: 1024, processing_status: "ready", processing_progress: 100,
    processing_stage: "done", source_width: 1920, source_height: 1080, source_bitrate: 0,
    video_codec: "h264", audio_codec: "aac", views_count: 12, likes_count: 3, favorites_count: 1,
    comments_count: 2, liked: false, favorited: false, subtitle_tracks: [], created_at: "2026-01-01T00:00:00Z", ...overrides };
}

export function pageFixture(items: Video[], overrides: Partial<VideoPage> = {}): VideoPage {
  return { items, page: 1, page_size: 36, total: items.length, has_next: false, ...overrides };
}
