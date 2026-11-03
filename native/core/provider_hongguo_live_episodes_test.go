package core

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// 联网回归：同一部剧连续切换多集都要拿到正片并能读到视频数据。
// 需要 CHECK_LIVE_PROVIDERS=true；可用 HONGGUO_LIVE_SERIES 指定剧 ID。
func TestLiveHongguoConsecutiveEpisodes(t *testing.T) {
	if os.Getenv("CHECK_LIVE_PROVIDERS") != "true" {
		t.Skip("set CHECK_LIVE_PROVIDERS=true to touch live 红果 APIs")
	}
	series := os.Getenv("HONGGUO_LIVE_SERIES")
	if series == "" {
		series = "7530119479875882009"
	}
	engine, err := newNativeEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := engine.downloader
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	_, chapters, err := d.fetchHongguoChapters(ctx, series)
	if err != nil || len(chapters) < 5 {
		t.Fatalf("chapters %d %v", len(chapters), err)
	}
	for index, chapter := range chapters[:5] {
		media, err := d.resolveHongguoMedia(ctx, Task{DramaID: providerDramaID(sourceHongguo, series), Chapter: chapter, Index: index + 1})
		if err != nil {
			t.Fatalf("episode %d: %v", index+1, err)
		}
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, media.URL, nil)
		request.Header.Set("Range", "bytes=0-1023")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("episode %d stream: %v", index+1, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusPartialContent && response.StatusCode != http.StatusOK {
			t.Fatalf("episode %d stream HTTP %d", index+1, response.StatusCode)
		}
		t.Logf("episode %d ok, %v, stream HTTP %d", index+1, media.Duration, response.StatusCode)
	}
}
