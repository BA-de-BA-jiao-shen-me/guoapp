package core

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func hongguoTestRevision(offset int64) int64 {
	date, _ := strconv.ParseInt(time.Now().UTC().Format("20060102"), 10, 64)
	return date*100 + 50 + offset
}

func hongguoTestConfig(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	var config map[string]any
	if err := json.Unmarshal(hongguoBundledRemoteConfig, &config); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(config)
	}
	raw, _ := json.Marshal(config)
	return string(raw)
}

func waitHongguoConfigSource(t *testing.T, d *Downloader, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		d.hongguoConfigState.mu.Lock()
		source, refreshing := d.hongguoConfigState.source, d.hongguoConfigState.refreshing
		d.hongguoConfigState.mu.Unlock()
		if source == want && !refreshing {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("config source never became %s", want)
}

func TestHongguoBundledConfigMatchesHostedCopyAndDefaults(t *testing.T) {
	hosted, err := os.ReadFile(filepath.Join("..", "..", "config", "hongguo-remote-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(hosted) != string(hongguoBundledRemoteConfig) {
		t.Fatal("config/hongguo-remote-config.json 与内置配置不一致")
	}
	config := hongguoBundledConfig()
	if config.Paths.VideoModel != hongguoVideoModelPath || config.Paths.MultiVideoModel != hongguoMediaPath {
		t.Fatalf("paths %+v", config.Paths)
	}
	if len(config.strategyIndexes) != len(hongguoPlaybackStrategies) || config.strategyIndexes[0] != 0 {
		t.Fatalf("strategies %v", config.strategyIndexes)
	}
	for _, address := range hongguoDefaultRemoteConfigURLs {
		if !hongguoRemoteConfigURLAllowed(address) {
			t.Fatal(address)
		}
	}
}

func TestHongguoRemoteConfigValidation(t *testing.T) {
	bad := map[string]func(map[string]any){
		"schema":      func(c map[string]any) { c["schema"] = 2 },
		"revision":    func(c map[string]any) { c["revision"] = 0 },
		"core level":  func(c map[string]any) { c["minCoreLevel"] = hongguoRemoteConfigCoreLevel + 1 },
		"probe low":   func(c map[string]any) { c["probeDevices"] = 0 },
		"probe high":  func(c map[string]any) { c["probeDevices"] = 21 },
		"ttl":         func(c map[string]any) { c["deviceTTLHours"] = 169 },
		"refresh":     func(c map[string]any) { c["refreshHours"] = 0 },
		"path host":   func(c map[string]any) { c["paths"].(map[string]any)["videoModel"] = "https://evil.example/x/" },
		"path dots":   func(c map[string]any) { c["paths"].(map[string]any)["multiVideoModel"] = "/novel/player/../x/" },
		"version":     func(c map[string]any) { c["playback"].(map[string]any)["versionCode"] = "71332&x=1" },
		"user agent":  func(c map[string]any) { c["playback"].(map[string]any)["userAgent"] = "evil\r\nX: 1" },
		"no strategy": func(c map[string]any) { c["strategies"] = []string{} },
		"all unknown": func(c map[string]any) { c["strategies"] = []string{"future_method"} },
		"wrong type":  func(c map[string]any) { c["probeDevices"] = "10" },
	}
	for name, mutate := range bad {
		if _, err := parseHongguoRemoteConfig([]byte(hongguoTestConfig(t, mutate))); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := parseHongguoRemoteConfig([]byte(`{"schema":1,` + strings.Repeat(" ", hongguoRemoteConfigMaxBytes) + `}`)); err == nil {
		t.Error("oversized accepted")
	}
	config, err := parseHongguoRemoteConfig([]byte(hongguoTestConfig(t, func(c map[string]any) {
		c["futureField"] = map[string]any{"x": 1}
		c["strategies"] = []string{"future_method", "multi_signed", "multi_signed", "multi_unsigned"}
	})))
	if err != nil || len(config.strategyIndexes) != 2 || hongguoPlaybackStrategies[config.strategyIndexes[0]].id != "multi_signed" {
		t.Fatalf("%v %+v", err, config)
	}
}

func TestHongguoRemoteConfigURLAllowed(t *testing.T) {
	for address, want := range map[string]bool{
		"https://cdn.example/a.json":      true,
		"http://127.0.0.1:8080/a.json":    true,
		"http://localhost/a.json":         true,
		"http://cdn.example/a.json":       false,
		"https://user:pass@cdn.example/a": false,
		"file:///etc/passwd":              false,
		"ftp://cdn.example/a.json":        false,
	} {
		if got := hongguoRemoteConfigURLAllowed(address); got != want {
			t.Errorf("%s: %v", address, got)
		}
	}
}

// 远程配置改了方式顺序后，不重新打包就生效；并写入本地缓存，重启后沿用。
func TestHongguoRemoteConfigChangesOrderAndPersists(t *testing.T) {
	remote := hongguoTestConfig(t, func(c map[string]any) {
		c["revision"] = hongguoTestRevision(0)
		c["strategies"] = []string{"video_model_signed", "multi_unsigned"}
	})
	var fetches atomic.Int32
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "mirror1.example":
			fetches.Add(1)
			return rankingHTTPResponse(request, 404, "missing"), nil
		case "mirror2.example":
			fetches.Add(1)
			return rankingHTTPResponse(request, 200, remote), nil
		}
		t.Errorf("unexpected %s", request.URL)
		return rankingHTTPResponse(request, 500, ""), nil
	})
	d.cfg.hongguoConfigURLs = []string{"https://mirror1.example/c.json", "https://mirror2.example/c.json"}
	if got := d.hongguoConfig(); got.Revision != hongguoBundledConfig().Revision {
		t.Fatalf("first call should return bundled immediately, got %d", got.Revision)
	}
	waitHongguoConfigSource(t, d, "https://mirror2.example/c.json")
	order := d.hongguoPlaybackStrategyOrder()
	if len(order) != 2 || hongguoPlaybackStrategies[order[0]].id != "video_model_signed" || fetches.Load() != 2 {
		t.Fatalf("order %v fetches %d", order, fetches.Load())
	}
	if d.hongguoConfig(); fetches.Load() != 2 {
		t.Fatal("refetched before refreshHours")
	}

	restarted := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		t.Errorf("unexpected %s", request.URL)
		return rankingHTTPResponse(request, 500, ""), nil
	})
	restarted.cfg.dataDir = d.cfg.dataDir
	if config := restarted.hongguoConfig(); config.Revision != hongguoTestRevision(0) || hongguoPlaybackStrategies[config.strategyIndexes[0]].id != "video_model_signed" {
		t.Fatalf("cache not used: %+v", config)
	}
}

func TestHongguoRemoteConfigIgnoresOlderRevisionAndInvalidPayload(t *testing.T) {
	older := hongguoTestConfig(t, func(c map[string]any) {
		c["revision"] = 2026010101
		c["strategies"] = []string{"multi_signed"}
	})
	for name, body := range map[string]string{"older": older, "invalid": `{"schema":1,"revision":2026111401,"strategies":["multi_signed"]}`, "html": "<html>"} {
		d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
			return rankingHTTPResponse(request, 200, body), nil
		})
		d.cfg.hongguoConfigURLs = []string{"https://mirror.example/c.json"}
		d.hongguoConfig()
		deadline := time.Now().Add(3 * time.Second)
		for {
			d.hongguoConfigState.mu.Lock()
			done := !d.hongguoConfigState.refreshing
			d.hongguoConfigState.mu.Unlock()
			if done || time.Now().After(deadline) {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if order := d.hongguoPlaybackStrategyOrder(); hongguoPlaybackStrategies[order[0]].id != "multi_unsigned" || len(order) != len(hongguoPlaybackStrategies) {
			t.Errorf("%s: order %v", name, order)
		}
	}
}

// 配置服务器卡住时不能阻塞取流：hongguoConfig 必须立刻返回。
func TestHongguoRemoteConfigNeverBlocksCaller(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		select {
		case <-release:
		case <-request.Context().Done():
		}
		return nil, context.Canceled
	})
	d.cfg.hongguoConfigURLs = []string{"https://slow.example/c.json"}
	start := time.Now()
	config := d.hongguoConfig()
	if time.Since(start) > 200*time.Millisecond || config.Revision != hongguoBundledConfig().Revision {
		t.Fatalf("blocked %v", time.Since(start))
	}
}

func hongguoGoodDeviceTransport(t *testing.T, registers *atomic.Int32) rankingTransport {
	return func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			registers.Add(1)
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		if request.Header.Get("X-Medusa") != "" {
			return rankingHTTPResponse(request, 200, ""), nil
		}
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"7694655440692448281":{"video_model":{"video_list":[{"main_url":"https://cdn.example/a.mp4","codec_type":"h264","definition":"720p","vheight":"1280"}]}}}}`), nil
	}
}

// 验证可用的设备保存到数据目录，重启后不重新注册直接复用；超过 deviceTTLHours 或失效后丢弃。
func TestHongguoPlaybackDevicePersistsAcrossRestart(t *testing.T) {
	var registers atomic.Int32
	d := rankingTestDownloader(t, hongguoGoodDeviceTransport(t, &registers))
	if _, err := d.resolveHongguoRegisteredMedia(context.Background(), "7694655440692448281"); err != nil || registers.Load() != 1 {
		t.Fatalf("%v registers %d", err, registers.Load())
	}
	saved, err := os.ReadFile(filepath.Join(d.cfg.dataDir, hongguoPlaybackDeviceFile))
	if err != nil || !strings.Contains(string(saved), "7123456789012345") || !strings.Contains(string(saved), "multi_unsigned") {
		t.Fatalf("%v %s", err, saved)
	}
	if info, _ := os.Stat(filepath.Join(d.cfg.dataDir, hongguoPlaybackDeviceFile)); info.Mode().Perm() != 0600 {
		t.Fatalf("mode %v", info.Mode())
	}

	restarted := rankingTestDownloader(t, hongguoGoodDeviceTransport(t, &registers))
	restarted.cfg.dataDir = d.cfg.dataDir
	if _, err := restarted.resolveHongguoRegisteredMedia(context.Background(), "7694655440692448281"); err != nil || registers.Load() != 1 {
		t.Fatalf("restart re-registered: %v registers %d", err, registers.Load())
	}

	var old hongguoSavedDevice
	_ = json.Unmarshal(saved, &old)
	old.VerifiedAt = time.Now().Add(-time.Duration(hongguoBundledConfig().DeviceTTLHours+1) * time.Hour)
	raw, _ := json.Marshal(old)
	_ = os.WriteFile(filepath.Join(d.cfg.dataDir, hongguoPlaybackDeviceFile), raw, 0600)
	expired := rankingTestDownloader(t, hongguoGoodDeviceTransport(t, &registers))
	expired.cfg.dataDir = d.cfg.dataDir
	if _, err := expired.resolveHongguoRegisteredMedia(context.Background(), "7694655440692448281"); err != nil || registers.Load() != 2 {
		t.Fatalf("expired device reused: %v registers %d", err, registers.Load())
	}
}

func TestHongguoFailedSavedDeviceIsForgotten(t *testing.T) {
	var registers atomic.Int32
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			registers.Add(1)
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789019999","install_id_str":"7123456789019998"}`), nil
		}
		return rankingHTTPResponse(request, 200, ""), nil
	})
	raw, _ := json.Marshal(hongguoSavedDevice{DeviceID: "7123456789012345", InstallID: "7123456789012346", Strategy: "multi_unsigned", VerifiedAt: time.Now()})
	path := filepath.Join(d.cfg.dataDir, hongguoPlaybackDeviceFile)
	_ = os.WriteFile(path, raw, 0600)
	_, err := d.resolveHongguoRegisteredMedia(context.Background(), "7694655440692448281")
	if err == nil || !strings.Contains(err.Error(), "已验证设备") {
		t.Fatalf("%v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("failed device still saved")
	}
}

func TestHongguoReuseDeviceCanBeDisabledRemotely(t *testing.T) {
	var registers atomic.Int32
	d := rankingTestDownloader(t, hongguoGoodDeviceTransport(t, &registers))
	config, err := parseHongguoRemoteConfig([]byte(hongguoTestConfig(t, func(c map[string]any) {
		c["revision"] = hongguoTestRevision(0)
		c["reuseDevice"] = false
	})))
	if err != nil {
		t.Fatal(err)
	}
	d.hongguoConfigState.loaded, d.hongguoConfigState.current = true, config
	for range 3 {
		if _, err := d.resolveHongguoRegisteredMedia(context.Background(), "7694655440692448281"); err != nil {
			t.Fatal(err)
		}
	}
	if registers.Load() != 3 {
		t.Fatalf("registers %d", registers.Load())
	}
	if _, err := os.Stat(filepath.Join(d.cfg.dataDir, hongguoPlaybackDeviceFile)); !os.IsNotExist(err) {
		t.Fatal("device saved although reuse disabled")
	}
}

// 远程配置的接口路径与播放版本参数实际用于请求。
func TestHongguoRemoteConfigPathsAndPlaybackParamsAreUsed(t *testing.T) {
	var sawPath, sawUA, sawVersion string
	d := rankingTestDownloader(t, func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "device_register") {
			return rankingHTTPResponse(request, 200, `{"code":0,"device_id_str":"7123456789012345","install_id_str":"7123456789012346"}`), nil
		}
		sawPath, sawUA, sawVersion = request.URL.Path, request.Header.Get("User-Agent"), request.URL.Query().Get("version_code")
		return rankingHTTPResponse(request, 200, `{"code":0,"data":{"7694655440692448281":{"video_model":{"video_list":[{"main_url":"https://cdn.example/a.mp4","codec_type":"h264","definition":"720p","vheight":"1280"}]}}}}`), nil
	})
	config, err := parseHongguoRemoteConfig([]byte(hongguoTestConfig(t, func(c map[string]any) {
		c["revision"] = hongguoTestRevision(0)
		c["strategies"] = []string{"multi_unsigned"}
		c["paths"].(map[string]any)["multiVideoModel"] = "/novel/player/multi_video_model/v2/"
		c["playback"].(map[string]any)["versionCode"] = "74532"
		c["playback"].(map[string]any)["userAgent"] = "com.phoenix.read/74532 (Linux; U; Android 14; test)"
	})))
	if err != nil {
		t.Fatal(err)
	}
	d.hongguoConfigState.loaded, d.hongguoConfigState.current = true, config
	if _, err := d.resolveHongguoRegisteredMedia(context.Background(), "7694655440692448281"); err != nil {
		t.Fatal(err)
	}
	if sawPath != "/novel/player/multi_video_model/v2/" || sawVersion != "74532" || !strings.HasPrefix(sawUA, "com.phoenix.read/74532") {
		t.Fatalf("%s %s %s", sawPath, sawVersion, sawUA)
	}
}
