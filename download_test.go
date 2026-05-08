package youtubedl

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testVideoURL = "https://youtu.be/OjNpRbNdR7E"

func TestDownloadVideo(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	video, err := client.GetVideo(testVideoURL)
	if err != nil {
		t.Fatalf("failed to get video: %v", err)
	}

	t.Logf("Video ID: %s", video.ID)
	t.Logf("Title: %s", video.Title)
	t.Logf("Author: %s", video.Author)
	t.Logf("Duration: %s", video.Duration)
	t.Logf("Views: %d", video.Views)
	t.Logf("Formats available: %d", len(video.Formats))

	if video.ID == "" {
		t.Error("video ID is empty")
	}
	if video.Title == "" {
		t.Error("video title is empty")
	}
	if video.Duration == 0 {
		t.Error("video duration is zero")
	}
	if len(video.Formats) == 0 {
		t.Fatal("no formats available for download")
	}
}

func TestDownloadVideoWithClient(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	clients := []string{"ANDROID_VR", "ANDROID", "WEB"}

	for _, clientName := range clients {
		t.Run(clientName, func(t *testing.T) {
			video, err := client.GetVideo(testVideoURL, WithClient(clientName))
			if err != nil {
				t.Fatalf("failed to get video with client %s: %v", clientName, err)
			}

			if video.ID == "" {
				t.Errorf("video ID is empty with client %s", clientName)
			}
			if len(video.Formats) == 0 {
				t.Errorf("no formats available with client %s", clientName)
			}

			t.Logf("[%s] Title: %s | Formats: %d", clientName, video.Title, len(video.Formats))
		})
	}
}

func TestDownloadVideoStream(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	video, err := client.GetVideo(testVideoURL)
	if err != nil {
		t.Fatalf("failed to get video: %v", err)
	}

	formats := video.Formats.Type("audio").WithAudioChannels()
	if len(formats) == 0 {
		t.Fatal("no audio formats available")
	}
	formats.Sort()
	format := &formats[0]

	t.Logf("Selected format: itag=%d, bitrate=%d, mimeType=%s, contentLength=%d",
		format.ItagNo, format.Bitrate, format.MimeType, format.ContentLength)

	stream, size, err := client.GetStream(video, format)
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}
	defer stream.Close()

	t.Logf("Stream size: %d bytes", size)

	buf := make([]byte, 64*1024)
	n, err := io.ReadFull(stream, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		t.Fatalf("failed to read stream: %v", err)
	}

	t.Logf("Read %d bytes from stream successfully", n)

	if n == 0 {
		t.Error("read 0 bytes from stream")
	}
}

func TestDownloadVideoToFile(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	video, err := client.GetVideo(testVideoURL)
	if err != nil {
		t.Fatalf("failed to get video: %v", err)
	}

	formats := video.Formats.Type("audio").WithAudioChannels()
	if len(formats) == 0 {
		t.Fatal("no audio formats available")
	}
	formats.Sort()
	format := &formats[0]

	t.Logf("Downloading format: itag=%d, mimeType=%s", format.ItagNo, format.MimeType)

	tmpDir, err := os.MkdirTemp("", "youtubedl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	keepDownload := os.Getenv("KEEP_DOWNLOAD") == "1"
	if !keepDownload {
		defer os.RemoveAll(tmpDir)
	} else {
		t.Logf("KEEP_DOWNLOAD=1, preserving downloaded file in: %s", tmpDir)
	}

	ext := ".mp4"
	if contains(format.MimeType, "webm") {
		ext = ".webm"
	} else if contains(format.MimeType, "opus") {
		ext = ".opus"
	}

	filename := filepath.Join(tmpDir, video.ID+ext)

	stream, size, err := client.GetStream(video, format)
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}
	defer stream.Close()

	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer file.Close()

	written, err := io.Copy(file, stream)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	t.Logf("Downloaded %d/%d bytes to %s", written, size, filename)

	info, err := os.Stat(filename)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("downloaded file is empty")
	}

	t.Logf("File size on disk: %d bytes", info.Size())
}

func TestDownloadVideoWithContext(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	video, err := client.GetVideoContext(ctx, testVideoURL)
	if err != nil {
		t.Fatalf("failed to get video with context: %v", err)
	}

	if video.ID == "" {
		t.Error("video ID is empty")
	}

	t.Logf("Successfully fetched video with context: %s", video.Title)
}

func TestDownloadVideoFormatFilters(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	video, err := client.GetVideo(testVideoURL)
	if err != nil {
		t.Fatalf("failed to get video: %v", err)
	}

	tests := []struct {
		name   string
		filter FormatList
	}{
		{"Video only", video.Formats.Type("video")},
		{"Audio only", video.Formats.Type("audio")},
		{"Audio with channels", video.Formats.WithAudioChannels()},
		{"720p quality", video.Formats.Quality("720p")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := len(tt.filter)
			t.Logf("%s: %d formats", tt.name, count)
			if count == 0 {
				t.Logf("WARNING: no formats matched filter %q (may be expected for some videos)", tt.name)
			}
		})
	}

	if len(video.Formats) > 0 {
		firstItag := video.Formats[0].ItagNo
		itagFormats := video.Formats.Itag(firstItag)
		t.Logf("Itag %d: %d formats", firstItag, len(itagFormats))
		if len(itagFormats) == 0 {
			t.Errorf("itag filter returned 0 formats for existing itag %d", firstItag)
		}
	}
}

func TestGetStreamURL(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	video, err := client.GetVideo(testVideoURL)
	if err != nil {
		t.Fatalf("failed to get video: %v", err)
	}

	if len(video.Formats) == 0 {
		t.Fatal("no formats available")
	}

	format := &video.Formats[0]
	streamURL, err := client.GetStreamURL(video, format)
	if err != nil {
		t.Fatalf("failed to get stream URL: %v", err)
	}

	if streamURL == "" {
		t.Error("stream URL is empty")
	}

	t.Logf("Stream URL obtained (length: %d chars)", len(streamURL))
}

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Full URL", "https://youtu.be/OjNpRbNdR7E", "OjNpRbNdR7E"},
		{"Watch URL", "https://www.youtube.com/watch?v=OjNpRbNdR7E", "OjNpRbNdR7E"},
		{"Short URL", "https://youtube.com/watch?v=OjNpRbNdR7E", "OjNpRbNdR7E"},
		{"Embed URL", "https://www.youtube.com/embed/OjNpRbNdR7E", "OjNpRbNdR7E"},
		{"Shorts URL", "https://www.youtube.com/shorts/OjNpRbNdR7E", "OjNpRbNdR7E"},
		{"Raw ID", "OjNpRbNdR7E", "OjNpRbNdR7E"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ExtractVideoID(tt.input)
			if err != nil {
				t.Fatalf("failed to extract video ID from %q: %v", tt.input, err)
			}
			if id != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, id)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
