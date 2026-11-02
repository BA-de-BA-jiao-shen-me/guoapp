package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// 上游 2026-11 起对不带签名的注册设备取流回空正文：必须先用注册设备号带六个签名头请求。
func TestHongguoPlaybackRequestSignsWithRegisteredDevice(t *testing.T) {
	const videoID = "7694655440692448281"
	var body map[string]any
	var sawRegister bool
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			sawRegister = true
			if request.Header.Get("Content-Type") != "application/octet-stream;tt-data=a" || request.Header.Get("x-ss-stub") == "" {
				t.Errorf("register headers %v", request.Header)
			}
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		if request.URL.Path != hongguoMediaPath {
			t.Errorf("path %s", request.URL.Path)
		}
		for _, name := range []string{"X-Gorgon", "X-Khronos", "X-Ladon", "X-Argus", "X-Helios", "X-Medusa"} {
			if request.Header.Get(name) == "" {
				t.Errorf("missing %s", name)
			}
		}
		if request.URL.Query().Get("device_id") != "7123456789012345" || request.URL.Query().Get("iid") != "7123456789012346" {
			t.Errorf("identity %s", request.URL.RawQuery)
		}
		if json.NewDecoder(request.Body).Decode(&body) != nil {
			t.Error("body")
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"`+videoID+`":{"video_model":{"video_list":[{"main_url":"https://cdn.example/plain.mp4","codec_type":"h264","definition":"1080p","vheight":"1920"}]}}}}`), nil
	})
	media, err := d.resolveHongguoAppMedia(context.Background(), videoID)
	if err != nil || media.URL != "https://cdn.example/plain.mp4" || media.Quality != 1080 || !sawRegister {
		t.Fatalf("%v register %v %+v", err, sawRegister, media)
	}
	param, _ := body["biz_param"].(map[string]any)
	mixed, _ := body["mixed_video_id_map"].(map[string]any)
	ids, _ := mixed["1004"].([]any)
	if param["video_platform"] != json.Number("1024") && param["video_platform"] != float64(1024) || param["need_all_video_definition"] != true || len(ids) != 1 || ids[0] != videoID {
		t.Fatalf("payload %+v", body)
	}
}

func TestHongguoRegisteredMediaFallsBackToUnsigned(t *testing.T) {
	const videoID = "7694655440692448281"
	var signed, unsigned int
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		if request.Header.Get("X-Argus") != "" {
			signed++
			return rankingHTTPResponse(request, 200, ""), nil
		}
		unsigned++
		if request.Header.Get("User-Agent") != hongguoPlaybackUserAgent {
			t.Errorf("ua %s", request.Header.Get("User-Agent"))
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"`+videoID+`":{"video_model":{"video_list":[{"main_url":"https://cdn.example/unsigned.mp4","codec_type":"h264","definition":"720p","vheight":"1280"}]}}}}`), nil
	})
	media, err := d.resolveHongguoRegisteredMedia(context.Background(), videoID)
	if err != nil || media.URL != "https://cdn.example/unsigned.mp4" || signed == 0 || unsigned == 0 {
		t.Fatalf("%v signed %d unsigned %d %+v", err, signed, unsigned, media)
	}
}

func TestHongguoRegisteredMediaRetriesRealPersonKey(t *testing.T) {
	const videoID = "7694655440692448281"
	var keys []string
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		var body map[string]any
		if json.NewDecoder(request.Body).Decode(&body) != nil {
			t.Fatal("body")
		}
		mixed, _ := body["mixed_video_id_map"].(map[string]any)
		for key := range mixed {
			keys = append(keys, key)
		}
		if _, comic := mixed["1004"]; comic {
			return rankingHTTPResponse(request, 200, ""), nil
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"`+videoID+`":{"video_model":{"video_list":[{"main_url":"https://cdn.example/live.mp4","codec_type":"h264","definition":"720p","vheight":"1280"}]}}}}`), nil
	})
	media, err := d.resolveHongguoRegisteredMedia(context.Background(), videoID)
	if err != nil || media.URL != "https://cdn.example/live.mp4" || len(keys) < 2 || keys[0] != "1004" {
		t.Fatalf("%v keys %v %+v", err, keys, media)
	}
}

func TestHongguoPlaybackReadsTopLevelKeyAndFallbackWithoutReferer(t *testing.T) {
	const videoID = "7694655440692448281"
	seed := []byte("seed-material-32bytes-pad-000000")
	sealed := sealHongguoTestURL("https://cdn.example/sealed.mp4", seed)
	var referer string
	var sawFallback bool
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		if request.URL.Path == "/gear" {
			sawFallback = true
			referer = request.Header.Get("Referer")
			payload, _ := json.Marshal(map[string]any{"video_list": []any{map[string]any{
				"main_url": sealed, "key_seed": base64.StdEncoding.EncodeToString(seed),
				"codec_type": "h264", "definition": "720p",
			}}})
			return rankingHTTPResponse(request, 200, string(payload)), nil
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"video_model":{"fallback_api":"https://gear.example/gear"}}}`), nil
	})
	media, err := d.resolveHongguoAppMedia(context.Background(), videoID)
	if err != nil || media.URL != "https://cdn.example/sealed.mp4" || media.Quality != 720 || !sawFallback || referer != "" {
		t.Fatalf("fallback %v referer %q saw %v %+v", err, referer, sawFallback, media)
	}
}

func sealHongguoTestURL(address string, keySeed []byte) string {
	sum := sha512.Sum512(keySeed)
	material := sha512.Sum512(append(sum[:], hongguoSeedURLConstant...))
	block, err := aes.NewCipher(material[:aes.BlockSize])
	if err != nil {
		panic(err)
	}
	plain := pkcs7Pad([]byte(address), aes.BlockSize)
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, material[aes.BlockSize:aes.BlockSize*2]).CryptBlocks(encrypted, plain)
	raw := append([]byte{0xA8, 0, 1, 0}, encrypted...)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestHongguoPlaybackDeviceIDStaysSixteenDigits(t *testing.T) {
	for range 20 {
		id, err := newHongguoPlaybackDeviceID()
		if err != nil || len(id) != 16 || strings.Trim(id, "0123456789") != "" {
			t.Fatal(id, err)
		}
	}
}
