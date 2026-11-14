package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// 2026-11-10 的上游行为：只有注册设备号 + 六个签名头 + 单集 video_model 有内容。
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
		if request.URL.Path != hongguoVideoModelPath {
			// 其他取流组合模拟成上游的空正文，确认会落到单集签名组合。
			return rankingHTTPResponse(request, 200, ""), nil
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
		// 与实测响应同形：data.video_model 是 JSON 字符串。
		model, _ := json.Marshal(map[string]any{"video_list": []any{map[string]any{"main_url": "https://cdn.example/plain.mp4", "codec_type": "h264", "definition": "1080p", "vheight": "1920"}}})
		payload, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]any{"video_model": string(model)}})
		return rankingHTTPResponse(request, 200, string(payload)), nil
	})
	media, err := d.resolveHongguoAppMedia(context.Background(), videoID)
	if err != nil || media.URL != "https://cdn.example/plain.mp4" || media.Quality != 1080 || !sawRegister {
		t.Fatalf("%v register %v %+v", err, sawRegister, media)
	}
	param, _ := body["biz_param"].(map[string]any)
	if body["video_id"] != videoID || param["video_platform"] != float64(3) || param["need_all_video_definition"] != true || param["video_id_type"] != float64(0) {
		t.Fatalf("payload %+v", body)
	}
	if _, legacy := body["mixed_video_id_map"]; legacy {
		t.Fatalf("payload still uses multi_video_model shape %+v", body)
	}
}

// 2026-11-13 实测：签名请求一律回空，注册设备 + 无签名 multi_video_model 可用。
// 默认顺序必须先试无签名组合，一次注册、一次请求就拿到流，不浪费签名请求。
func TestHongguoRegisteredMediaPrefersUnsignedMulti(t *testing.T) {
	const videoID = "7694655440692448281"
	var signed, unsigned, registers int
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			registers++
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		if request.Header.Get("X-Argus") != "" {
			signed++
			return rankingHTTPResponse(request, 200, ""), nil
		}
		unsigned++
		if request.Header.Get("User-Agent") != hongguoPlaybackUserAgent || request.URL.Path != hongguoMediaPath {
			t.Errorf("ua %s path %s", request.Header.Get("User-Agent"), request.URL.Path)
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"`+videoID+`":{"video_model":{"video_list":[{"main_url":"https://cdn.example/unsigned.mp4","codec_type":"h264","definition":"720p","vheight":"1280"}]}}}}`), nil
	})
	media, err := d.resolveHongguoRegisteredMedia(context.Background(), videoID)
	if err != nil || media.URL != "https://cdn.example/unsigned.mp4" || signed != 0 || unsigned != 1 || registers != 1 {
		t.Fatalf("%v signed %d unsigned %d registers %d %+v", err, signed, unsigned, registers, media)
	}
}

func TestHongguoRegisteredMediaRetriesRealPersonKey(t *testing.T) {
	const videoID = "7694655440692448281"
	var keys []string
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		if request.URL.Path == hongguoVideoModelPath {
			return rankingHTTPResponse(request, 200, ""), nil
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

// 模拟实测到的上游行为：每台注册设备只有第一次取流有内容，之后回空正文。
// 连续切换多集都必须能取到流（每集换一台新设备）。
func TestHongguoRegisteredMediaUsesFreshDevicePerEpisode(t *testing.T) {
	videos := []string{"7694655440692448281", "7694655440692448282", "7694655440692448283", "7694655440692448284"}
	registers := 0
	used := map[string]bool{}
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			registers++
			id := fmt.Sprintf("71234567890%05d", registers)
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"`+id+`","install_id_str":"`+id+`"}`), nil
		}
		device := request.URL.Query().Get("device_id")
		if used[device] {
			return rankingHTTPResponse(request, 200, ""), nil
		}
		used[device] = true
		raw, _ := io.ReadAll(request.Body)
		video := ""
		for _, candidate := range videos {
			if strings.Contains(string(raw), candidate) {
				video = candidate
			}
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"`+video+`":{"video_model":{"video_list":[{"main_url":"https://cdn.example/`+video+`.mp4","codec_type":"h264","definition":"720p","vheight":"1280"}]}}}}`), nil
	})
	for _, video := range videos {
		media, err := d.resolveHongguoRegisteredMedia(context.Background(), video)
		if err != nil || media.URL != "https://cdn.example/"+video+".mp4" {
			t.Fatalf("%s: %v %+v", video, err, media)
		}
	}
	if registers != len(videos) {
		t.Fatalf("registers %d", registers)
	}
}

// 实测只有分支 2 的签名能被上游接受，签名时必须调整 _rticket 落到分支 2。
func TestHongguoPlaybackSignatureAlwaysUsesAcceptedBranch(t *testing.T) {
	body := []byte(`{"mixed_video_id_map":{"1004":["7694655440692448281"]}}`)
	for i := 0; i < 50; i++ {
		request, _ := http.NewRequest(http.MethodPost, "https://example.test"+hongguoMediaPath+"?aid=8662&device_id=1&_rticket="+fmt.Sprint(i), nil)
		now := time.Unix(1793712000+int64(i), 0)
		signHongguoPlaybackRequest(request, body, now)
		if got := hongguoSignBranch(request.URL.RawQuery, body, uint32(now.Unix())); got != hongguoSignValidBranch {
			t.Fatalf("branch %d for %s", got, request.URL.RawQuery)
		}
	}
}

// 模拟 2026-11-10 实测到的上游行为：multi_video_model 对任何设备都回空正文；
// video_model 每台新注册设备只有第一次有内容。连续多集都必须取到流，
// 且单集接口必须是每台设备的第一个取流请求（否则设备会被浪费掉）。
func TestHongguoRegisteredMediaUsesSingleVideoModelFirst(t *testing.T) {
	videos := []string{"7694655440692448281", "7694655440692448282", "7694655440692448283", "7694655440692448284", "7694655440692448285"}
	registers, multiCalls := 0, 0
	used := map[string]bool{}
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			registers++
			id := fmt.Sprintf("71234567890%05d", registers)
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"`+id+`","install_id_str":"`+id+`"}`), nil
		}
		device := request.URL.Query().Get("device_id")
		first := !used[device]
		used[device] = true
		if request.URL.Path == hongguoMediaPath {
			multiCalls++
			return rankingHTTPResponse(request, 200, ""), nil
		}
		if request.URL.Path != hongguoVideoModelPath || !first || request.Header.Get("X-Medusa") == "" {
			return rankingHTTPResponse(request, 200, ""), nil
		}
		var body map[string]any
		if json.NewDecoder(request.Body).Decode(&body) != nil {
			t.Fatal("body")
		}
		video, _ := body["video_id"].(string)
		model, _ := json.Marshal(map[string]any{"video_list": []any{map[string]any{"main_url": "https://cdn.example/" + video + ".mp4", "codec_type": "h264", "definition": "720p", "vheight": "1280"}}})
		payload, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]any{"video_model": string(model)}})
		return rankingHTTPResponse(request, 200, string(payload)), nil
	})
	for _, video := range videos {
		media, err := d.resolveHongguoMedia(context.Background(), Task{DramaID: providerDramaID(sourceHongguo, "7530119479875882009"), Chapter: Chapter{VideoURL: "hongguo-cenc://" + video}})
		if err != nil || media.URL != "https://cdn.example/"+video+".mp4" {
			t.Fatalf("%s: %v %+v", video, err, media)
		}
	}
	// 第一集先试默认的多集无签名（一台设备、两个接入域名），失败后单集签名成功并被记住；
	// 之后每集直接走单集签名，每集只注册一台设备。
	if registers != len(videos)+1 || multiCalls != 2 {
		t.Fatalf("registers %d multi calls %d", registers, multiCalls)
	}
}

// 上游中途换可用组合（单集签名 → 多集无签名）时，连续换集都要能取到流，并且会改记新组合。
func TestHongguoRegisteredMediaFollowsUpstreamRotation(t *testing.T) {
	videos := []string{"7694655440692448281", "7694655440692448282", "7694655440692448283", "7694655440692448284"}
	signedEra := true
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		raw, _ := io.ReadAll(request.Body)
		isSigned := request.Header.Get("X-Medusa") != ""
		ok := (signedEra && isSigned && request.URL.Path == hongguoVideoModelPath) || (!signedEra && !isSigned && request.URL.Path == hongguoMediaPath)
		if !ok {
			return rankingHTTPResponse(request, 200, ""), nil
		}
		video := ""
		for _, candidate := range videos {
			if strings.Contains(string(raw), candidate) {
				video = candidate
			}
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"`+video+`":{"video_model":{"video_list":[{"main_url":"https://cdn.example/`+video+`.mp4","codec_type":"h264","definition":"720p","vheight":"1280"}]}}}}`), nil
	})
	for index, video := range videos {
		signedEra = index < 2
		media, err := d.resolveHongguoRegisteredMedia(context.Background(), video)
		if err != nil || media.URL != "https://cdn.example/"+video+".mp4" {
			t.Fatalf("%s: %v %+v", video, err, media)
		}
	}
	if order := d.hongguoPlaybackStrategyOrder(); hongguoPlaybackStrategies[order[0]].signed {
		t.Fatalf("still prefers %s", hongguoPlaybackStrategies[order[0]].name)
	}
}

// 所有组合都回空时，错误里要逐项写出每种组合的结果，方便用户反馈后定位。
func TestHongguoRegisteredMediaReportsEveryStrategy(t *testing.T) {
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		return rankingHTTPResponse(request, 200, ""), nil
	})
	_, err := d.resolveHongguoRegisteredMedia(context.Background(), "7694655440692448281")
	if err == nil {
		t.Fatal("expected error")
	}
	for _, strategy := range hongguoPlaybackStrategies {
		if !strings.Contains(err.Error(), strategy.name) {
			t.Fatalf("missing %s in %v", strategy.name, err)
		}
	}
}

// 2026-11-14 实测：新注册设备约一半一律回空、一半能取任意集。模拟第 1、4、7… 台设备坏、其余设备好，
// 连续换集时要复用验证可用的设备，而不是每集重新抽设备。
func TestHongguoRegisteredMediaReusesVerifiedDevice(t *testing.T) {
	videos := []string{"7694655440692448281", "7694655440692448282", "7694655440692448283", "7694655440692448284", "7694655440692448285", "7694655440692448286"}
	var mu sync.Mutex
	registers := 0
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		if strings.Contains(request.URL.Path, "device_register") {
			registers++
			id := fmt.Sprintf("71234567890%05d", registers)
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"`+id+`","install_id_str":"`+id+`"}`), nil
		}
		device := request.URL.Query().Get("device_id")
		number, _ := strconv.Atoi(device[len(device)-5:])
		if number%3 == 1 || request.Header.Get("X-Medusa") != "" || request.URL.Path != hongguoMediaPath {
			return rankingHTTPResponse(request, 200, ""), nil
		}
		raw, _ := io.ReadAll(request.Body)
		video := ""
		for _, candidate := range videos {
			if strings.Contains(string(raw), candidate) {
				video = candidate
			}
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"`+video+`":{"video_model":{"video_list":[{"main_url":"https://cdn.example/`+video+`.mp4","codec_type":"h264","definition":"720p","vheight":"1280"}]}}}}`), nil
	})
	var wait sync.WaitGroup
	errs := make(chan error, len(videos))
	for _, video := range videos {
		wait.Add(1)
		go func(video string) {
			defer wait.Done()
			media, err := d.resolveHongguoRegisteredMedia(context.Background(), video)
			if err == nil && media.URL != "https://cdn.example/"+video+".mp4" {
				err = fmt.Errorf("wrong media %s", media.URL)
			}
			errs <- err
		}(video)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if registers > 3 {
		t.Fatalf("registered %d devices for %d episodes", registers, len(videos))
	}
}

func TestHongguoProbeStrategyInterleavesPreferred(t *testing.T) {
	order := []int{3, 0, 1, 2, 4}
	var got []int
	for probe := 0; probe < hongguoPlaybackProbeDevices; probe++ {
		got = append(got, hongguoProbeStrategy(order, probe))
	}
	want := []int{3, 0, 3, 1, 3, 2, 3, 4, 3, 3}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// 实测（2026-11-14）：官网播放页只对前 3 集返回 200，第 4 集起 HTTP 404。
func TestHongguoWebMediaExplainsMissingPreviewPage(t *testing.T) {
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		return rankingHTTPResponse(request, 404, "not found"), nil
	})
	_, err := d.resolveHongguoWebMedia(context.Background(), "7530119479875882009", "7530164917408631870")
	if err == nil || !strings.Contains(err.Error(), "只公开前几集") {
		t.Fatalf("%v", err)
	}
}
