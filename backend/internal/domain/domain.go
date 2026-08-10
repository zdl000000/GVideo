package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrInvalidInput     = errors.New("invalid input")
	ErrInvalidSession   = errors.New("invalid session")
	ErrSubtitleExists   = errors.New("subtitle already exists")
	ErrVideoProcessing  = errors.New("video is processing")
	ErrRetryUnavailable = errors.New("retry unavailable")
)

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Bio       string    `json:"bio"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatorProfile struct {
	ID             int64     `json:"id"`
	Username       string    `json:"username"`
	Bio            string    `json:"bio"`
	FollowersCount int64     `json:"followers_count"`
	FollowingCount int64     `json:"following_count"`
	VideosCount    int64     `json:"videos_count"`
	Followed       bool      `json:"followed"`
	CreatedAt      time.Time `json:"created_at"`
}

type Session struct {
	User      User
	CSRFToken string
	ExpiresAt time.Time
}

type Video struct {
	ID                int64           `json:"id"`
	UserID            int64           `json:"user_id"`
	Username          string          `json:"username"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	Category          string          `json:"category"`
	VideoURL          string          `json:"video_url"`
	HLSURL            string          `json:"hls_url"`
	CoverURL          string          `json:"cover_url"`
	MimeType          string          `json:"mime_type"`
	DurationSeconds   float64         `json:"duration_seconds"`
	SizeBytes         int64           `json:"size_bytes"`
	ProcessingStatus  string          `json:"processing_status"`
	SourceWidth       int             `json:"source_width"`
	SourceHeight      int             `json:"source_height"`
	SourceBitrate     int64           `json:"source_bitrate"`
	VideoCodec        string          `json:"video_codec"`
	AudioCodec        string          `json:"audio_codec"`
	ProcessingError   string          `json:"-"`
	ProcessingMessage string          `json:"processing_error,omitempty"`
	ProcessedAt       *time.Time      `json:"processed_at,omitempty"`
	ViewsCount        int64           `json:"views_count"`
	LikesCount        int64           `json:"likes_count"`
	FavoritesCount    int64           `json:"favorites_count"`
	CommentsCount     int64           `json:"comments_count"`
	Liked             bool            `json:"liked"`
	Favorited         bool            `json:"favorited"`
	SubtitleTracks    []SubtitleTrack `json:"subtitle_tracks"`
	CreatedAt         time.Time       `json:"created_at"`
}

type SubtitleTrack struct {
	ID        int64  `json:"id"`
	Language  string `json:"language"`
	Label     string `json:"label"`
	URL       string `json:"url"`
	IsDefault bool   `json:"is_default"`
}

type MediaMetadata struct {
	DurationSeconds float64
	Width           int
	Height          int
	Bitrate         int64
	VideoCodec      string
	AudioCodec      string
}

type MediaOutput struct {
	Metadata      MediaMetadata
	HLSMasterPath string
}

type TranscodingJob struct {
	ID          int64
	VideoID     int64
	VideoPath   string
	Attempts    int
	AvailableAt time.Time
}

type Comment struct {
	ID        int64     `json:"id"`
	VideoID   int64     `json:"video_id"`
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type VideoFilter struct {
	Query           string
	Category        string
	Sort            string
	UserID          int64
	FollowingUserID int64
	Limit           int
	Offset          int
}

type VideoPage struct {
	Items    []Video `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	Total    int64   `json:"total"`
	HasNext  bool    `json:"has_next"`
}

type UpdateVideo struct {
	Title       string
	Description string
	Category    string
	CoverPath   *string
}

type VideoAssets struct {
	VideoPath     string
	CoverPath     string
	HLSMasterPath string
	SubtitlePaths []string
}

type NewVideo struct {
	UserID          int64
	Title           string
	Description     string
	Category        string
	VideoPath       string
	CoverPath       string
	MimeType        string
	DurationSeconds float64
	SizeBytes       int64
}

type NewSubtitle struct {
	Language  string
	Label     string
	Path      string
	IsDefault bool
}
