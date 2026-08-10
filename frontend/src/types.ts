export interface User {
  id: number;
  username: string;
  bio: string;
  created_at: string;
}

export interface CreatorProfile extends User {
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
  title: string;
  description: string;
  category: string;
  video_url: string;
  hls_url: string;
  cover_url: string;
  mime_type: string;
  duration_seconds: number;
  size_bytes: number;
  processing_status: "pending" | "processing" | "ready" | "failed";
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

export interface Comment {
  id: number;
  video_id: number;
  user_id: number;
  username: string;
  content: string;
  created_at: string;
}

export interface AuthPayload {
  user: User;
  csrf_token: string;
}
