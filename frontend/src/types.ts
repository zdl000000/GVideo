export interface User {
  id: number;
  username: string;
  bio: string;
  avatar_url: string;
  is_admin: boolean;
  created_at: string;
}

export interface CreatorProfile extends Omit<User, "is_admin"> {
  followers_count: number;
  following_count: number;
  videos_count: number;
  followed: boolean;
}

export interface SubtitleTrack {
  id: number;
  language: string;
  label: string;
  url: string;
  is_default: boolean;
}

export interface Video {
  id: number;
  user_id: number;
  username: string;
  avatar_url: string;
  title: string;
  description: string;
  category: string;
  visibility: "public" | "unlisted" | "private";
  video_url: string;
  hls_url: string;
  cover_url: string;
  mime_type: string;
  duration_seconds: number;
  size_bytes: number;
  processing_status: "pending" | "processing" | "ready" | "failed";
  processing_progress: number;
  processing_stage: string;
  source_width: number;
  source_height: number;
  source_bitrate: number;
  video_codec: string;
  audio_codec: string;
  processing_error?: string;
  processed_at?: string;
  views_count: number;
  likes_count: number;
  favorites_count: number;
  comments_count: number;
  liked: boolean;
  favorited: boolean;
  subtitle_tracks: SubtitleTrack[];
  created_at: string;
}

export interface VideoPage {
  items: Video[];
  page: number;
  page_size: number;
  total: number;
  has_next: boolean;
}

export type VideoReportStatus = "pending" | "reviewed" | "resolved" | "dismissed";

export interface VideoReport {
  id: number;
  video_id: number;
  video_title: string;
  video_author_id: number;
  video_author: string;
  user_id: number;
  reporter_username: string;
  reason: string;
  detail: string;
  status: VideoReportStatus;
  created_at: string;
  updated_at: string;
}

export interface VideoReportPage {
  items: VideoReport[];
  page: number;
  page_size: number;
  total: number;
  has_next: boolean;
}

export interface Comment {
  id: number;
  video_id: number;
  user_id: number;
  username: string;
  avatar_url: string;
  content: string;
  created_at: string;
}

export interface CreatorStats {
  videos_count: number;
  followers_count: number;
  views_count: number;
  likes_count: number;
  favorites_count: number;
  comments_count: number;
  public_count: number;
  unlisted_count: number;
  private_count: number;
  processing_count: number;
  recent_videos: Video[];
}

export interface AuthPayload {
  user: User;
  csrf_token: string;
}

export interface Notification {
  id: number;
  type: "follow" | "like" | "favorite" | "comment" | "processing_ready" | "processing_failed";
  actor_id?: number;
  actor_username?: string;
  actor_avatar_url?: string;
  video_id?: number;
  video_title?: string;
  comment_id?: number;
  comment_preview?: string;
  read_at?: string;
  created_at: string;
}

export interface NotificationPage {
  items: Notification[];
  page: number;
  page_size: number;
  total: number;
  has_next: boolean;
  unread_count: number;
}
