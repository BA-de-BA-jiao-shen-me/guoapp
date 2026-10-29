package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestHongguoAppRoutesPreferDocumentedHosts(t *testing.T) {
	var seen []string
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		seen = append(seen, request.URL.Host+" "+request.URL.Path)
		if request.URL.Query().Get("aid") != "8662" || request.URL.Query().Get("version_name") != "7.3.5.32" || request.URL.Query().Get("version_code") != "73532" {
			t.Errorf("version query changed: %s", request.URL.RawQuery)
		}
		switch request.URL.Host + request.URL.Path {
		case "api5-normal-sinfonlineb.fqnovel.com" + hongguoLandpagePath, "api5-normal-lq.fqnovel.com" + hongguoMediaPath:
			return rankingHTTPResponse(request, 200, ""), nil
		case "api5-normal-lq.fqnovel.com" + hongguoDetailPath:
			return rankingHTTPResponse(request, 200, `{"code":"110001"}`), nil
		default:
			return rankingHTTPResponse(request, 200, `{"code":0,"data":{}}`), nil
		}
	})
	if _, err := d.hongguoAppRequest(context.Background(), http.MethodPost, hongguoLandpagePath, nil, map[string]any{"offset": 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.hongguoAppRequestOrdered(context.Background(), http.MethodPost, hongguoMediaPath, nil, map[string]any{"video_id": "1"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := d.hongguoAppRequestOrdered(context.Background(), http.MethodPost, hongguoDetailPath, nil, map[string]any{"series_id": "1"}, true); err == nil || !strings.Contains(err.Error(), "110001") {
		t.Fatal(err)
	}
	got := strings.Join(seen, "\n")
	want := strings.Join([]string{
		"api5-normal-sinfonlineb.fqnovel.com " + hongguoLandpagePath,
		"api5-normal-lq.fqnovel.com " + hongguoLandpagePath,
		"api5-normal-lq.fqnovel.com " + hongguoMediaPath,
		"api5-normal-sinfonlineb.fqnovel.com " + hongguoMediaPath,
		"api5-normal-lq.fqnovel.com " + hongguoDetailPath,
	}, "\n")
	if got != want {
		t.Fatalf("route order:\n%s", got)
	}
}

func TestHongguoDetailReadsSeriesKeyedVideoData(t *testing.T) {
	const series = "1111111111111111111"
	const video = "2222222222222222222"
	result := map[string]any{"data": map[string]any{series: map[string]any{"video_data": map[string]any{
		"series_id": series, "series_title": "样本", "episode_cnt": "1",
		"video_list": []any{map[string]any{"vid": video, "vid_index": "1"}},
	}}}}
	entry, err := parseHongguoAppDetail(result, series)
	if err != nil || len(entry.Chapters) != 1 || entry.Chapters[0].VideoURL != "hongguo-cenc://"+video {
		t.Fatalf("series-keyed detail: %+v %v", entry, err)
	}
	model, err := hongguoVideoModel(map[string]any{"data": map[string]any{video: map[string]any{"video_model": map[string]any{"video_list": []any{}}}}}, video)
	if err != nil || model["video_list"] == nil {
		t.Fatalf("video-keyed model: %+v %v", model, err)
	}
}
