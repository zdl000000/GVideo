package httpapi

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/modules/comments"
	"gvideo/backend/internal/modules/moderation"
	"gvideo/backend/internal/modules/notifications"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
	"gvideo/backend/internal/service"
)

// A minimal MP4 signature the standard library's sniffer recognizes.
var quotaTestMP4 = []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom<\x06t\xbfmdat")

func TestUploadQuotaReturnsRequestEntityTooLarge(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "quota-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Config{
		MediaDir:              filepath.Join(dir, "media"),
		SessionTTL:            time.Hour,
		MaxUploadBytes:        1 << 20,
		UserStorageQuotaBytes: int64(len(quotaTestMP4)) + 5,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(service.New(repository.New(db), cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), comments.NewService(comments.NewRepository(db), logger), cfg, logger).Routes()
	user := registerTestUser(t, handler, "quota_http_user")

	upload := func(title string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("title", title)
		_ = writer.WriteField("description", "quota contract test")
		_ = writer.WriteField("category", service.Categories[0])
		_ = writer.WriteField("visibility", "public")
		part, err := writer.CreateFormFile("video", "sample.mp4")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(quotaTestMP4); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/v1/videos", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("X-CSRF-Token", user.CSRFToken)
		request.AddCookie(user.Cookie)
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, request)
		return result
	}

	first := upload("首个小视频")
	if first.Code != http.StatusCreated {
		t.Fatalf("first upload status=%d body=%s", first.Code, first.Body.String())
	}
	second := upload("超出配额")
	if second.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("quota status=%d body=%s", second.Code, second.Body.String())
	}
	if !bytes.Contains(second.Body.Bytes(), []byte("存储空间已用完")) {
		t.Fatalf("quota message missing: %s", second.Body.String())
	}
}
