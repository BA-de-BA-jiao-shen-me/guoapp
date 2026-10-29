package core

import (
	"context"
	"net/http"
	"testing"
)

func TestHongguoPureSignMatchesReference(t *testing.T) {
	sum := hongguoSM3([]byte("abc"))
	if got := hexOf(sum[:]); got != "66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0" {
		t.Fatalf("sm3 %s", got)
	}
	query := "aid=8662&device_id=1000000000000001&version_name=7.3.5.32"
	body := []byte(`{"video_id":"1","content_type":1}`)
	if got := hongguoXGorgon(query, body, 1700000000, 0x1234); got != "8404341240012c36906f8606aa7a1364e71df6105a46126993de" {
		t.Fatalf("gorgon %s", got)
	}
	if got := hongguoHelios(1700000000, 0x01020304); got != "BAMCAaBPXV/+KNBRDXGiYSJtXJS2Wy6kFGNFrIJDsc+FCdVp" {
		t.Fatalf("helios %s", got)
	}
	if got := hongguoSignBranch(query, body, 1700000000); got != 2 {
		t.Fatalf("branch %d", got)
	}
}

func TestHongguoCatalogRequestOmitsPlaybackSignatures(t *testing.T) {
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("X-Helios") != "" || request.Header.Get("X-Medusa") != "" {
			t.Error("catalog request used the playback signature")
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"video_data":[],"has_more":false,"next_offset":0}}`), nil
	})
	if _, err := d.hongguoAppRequest(context.Background(), http.MethodPost, hongguoLandpagePath, nil, map[string]any{"offset": 0}); err != nil {
		t.Fatal(err)
	}
}

func hexOf(value []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(value)*2)
	for index, current := range value {
		out[index*2] = digits[current>>4]
		out[index*2+1] = digits[current&15]
	}
	return string(out)
}
