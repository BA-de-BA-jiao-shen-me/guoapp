package core

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 联网回归：同一部剧连续切换多集都要拿到正片并能读到视频数据。
// 需要 CHECK_LIVE_PROVIDERS=true；可用 HONGGUO_LIVE_SERIES 指定剧 ID（逗号分隔多部），
// HONGGUO_LIVE_EPISODES 指定每部连续取几集（默认 5）。
func TestLiveHongguoConsecutiveEpisodes(t *testing.T) {
	if os.Getenv("CHECK_LIVE_PROVIDERS") != "true" {
		t.Skip("set CHECK_LIVE_PROVIDERS=true to touch live 红果 APIs")
	}
	seriesList := os.Getenv("HONGGUO_LIVE_SERIES")
	if seriesList == "" {
		seriesList = "7530119479875882009"
	}
	episodes, _ := strconv.Atoi(os.Getenv("HONGGUO_LIVE_EPISODES"))
	if episodes <= 0 {
		episodes = 5
	}
	engine, err := newNativeEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := engine.downloader
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for _, series := range strings.Split(seriesList, ",") {
		_, chapters, err := d.fetchHongguoChapters(ctx, series)
		if err != nil || len(chapters) < episodes {
			t.Fatalf("%s chapters %d %v", series, len(chapters), err)
		}
		for index, chapter := range chapters[:episodes] {
			media, err := d.resolveHongguoMedia(ctx, Task{DramaID: providerDramaID(sourceHongguo, series), Chapter: chapter, Index: index + 1})
			if err != nil {
				t.Fatalf("%s episode %d: %v", series, index+1, err)
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
			t.Logf("%s episode %d ok, %v, stream HTTP %d", series, index+1, media.Duration, response.StatusCode)
		}
	}
}
