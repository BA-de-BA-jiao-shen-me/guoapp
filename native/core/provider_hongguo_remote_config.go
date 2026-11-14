package core

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// hongguoRemoteConfigCoreLevel 是本核心能理解的配置能力等级；配置的 minCoreLevel 高于它时整份配置不生效。
const hongguoRemoteConfigCoreLevel = 1

const (
	hongguoRemoteConfigSchema    = 1
	hongguoRemoteConfigMaxBytes  = 16 << 10
	hongguoRemoteConfigTimeout   = 8 * time.Second
	hongguoRemoteConfigCacheFile = "hongguo_remote_config.json"
	hongguoPlaybackDeviceFile    = "hongguo_playback_device.json"
	hongguoRemoteConfigURLsEnv   = "DUANJU_HONGGUO_CONFIG_URLS"
)

// hongguoDefaultRemoteConfigURLs 依次尝试，第一份通过校验的配置生效。前四个是 GitHub 与 jsDelivr 官方地址，
// 最后两个是国内常用的第三方 GitHub 加速代理，只在前面都失败时使用；内容同样严格校验，revision 有上限，
// 代理即使返回恶意内容也只能在允许范围内调整取流方式，且最多影响到 revision 上限对应的日期。
var hongguoDefaultRemoteConfigURLs = []string{
	"https://raw.githubusercontent.com/BA-de-BA-jiao-shen-me/guoapp/main/config/hongguo-remote-config.json",
	"https://cdn.jsdelivr.net/gh/BA-de-BA-jiao-shen-me/guoapp@main/config/hongguo-remote-config.json",
	"https://gcore.jsdelivr.net/gh/BA-de-BA-jiao-shen-me/guoapp@main/config/hongguo-remote-config.json",
	"https://fastly.jsdelivr.net/gh/BA-de-BA-jiao-shen-me/guoapp@main/config/hongguo-remote-config.json",
	"https://gh-proxy.com/https://raw.githubusercontent.com/BA-de-BA-jiao-shen-me/guoapp/main/config/hongguo-remote-config.json",
	"https://ghfast.top/https://raw.githubusercontent.com/BA-de-BA-jiao-shen-me/guoapp/main/config/hongguo-remote-config.json",
}

// hongguoMaxRevision 要求 revision 采用 YYYYMMDDNN 格式，且日期不晚于两天后，
// 防止一份写了超大 revision 的配置永久压住后续的正常更新。
func hongguoMaxRevision(now time.Time) int64 {
	date, _ := strconv.ParseInt(now.UTC().Add(48*time.Hour).Format("20060102"), 10, 64)
	return date*100 + 99
}

//go:embed hongguo_remote_config.json
var hongguoBundledRemoteConfig []byte

type hongguoRemoteConfig struct {
	Schema         int      `json:"schema"`
	Revision       int64    `json:"revision"`
	MinCoreLevel   int      `json:"minCoreLevel"`
	Strategies     []string `json:"strategies"`
	ProbeDevices   int      `json:"probeDevices"`
	ReuseDevice    bool     `json:"reuseDevice"`
	DeviceTTLHours int      `json:"deviceTTLHours"`
	RefreshHours   int      `json:"refreshHours"`
	Paths          struct {
		VideoModel      string `json:"videoModel"`
		MultiVideoModel string `json:"multiVideoModel"`
	} `json:"paths"`
	Playback struct {
		VersionCode string `json:"versionCode"`
		VersionName string `json:"versionName"`
		UserAgent   string `json:"userAgent"`
	} `json:"playback"`
	strategyIndexes []int
}

var (
	hongguoConfigPathPattern    = regexp.MustCompile(`^/novel/player/[a-z0-9_]+(/[a-z0-9_]+)*/$`)
	hongguoConfigVersionCode    = regexp.MustCompile(`^[0-9]{5,6}$`)
	hongguoConfigVersionName    = regexp.MustCompile(`^[0-9]{1,2}(\.[0-9]{1,3}){3}$`)
	hongguoConfigUserAgentRunes = regexp.MustCompile(`^com\.phoenix\.read/[0-9]{5,6} \([\x20-\x7e]{1,240}\)$`)
)

func parseHongguoRemoteConfig(raw []byte) (*hongguoRemoteConfig, error) {
	if len(raw) == 0 || len(raw) > hongguoRemoteConfigMaxBytes {
		return nil, errors.New("红果远程配置大小无效")
	}
	var config hongguoRemoteConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, errors.New("红果远程配置不是有效的 JSON")
	}
	switch {
	case config.Schema != hongguoRemoteConfigSchema:
		return nil, fmt.Errorf("红果远程配置版本 %d 不受支持", config.Schema)
	case config.Revision < 2026010100 || config.Revision > hongguoMaxRevision(time.Now()):
		return nil, errors.New("红果远程配置 revision 无效")
	case config.MinCoreLevel > hongguoRemoteConfigCoreLevel:
		return nil, fmt.Errorf("红果远程配置需要更新的应用（核心等级 %d）", config.MinCoreLevel)
	case config.ProbeDevices < 1 || config.ProbeDevices > 20:
		return nil, errors.New("红果远程配置 probeDevices 超出范围")
	case config.DeviceTTLHours < 0 || config.DeviceTTLHours > 168:
		return nil, errors.New("红果远程配置 deviceTTLHours 超出范围")
	case config.RefreshHours < 1 || config.RefreshHours > 168:
		return nil, errors.New("红果远程配置 refreshHours 超出范围")
	case !hongguoConfigPathPattern.MatchString(config.Paths.VideoModel) || len(config.Paths.VideoModel) > 96,
		!hongguoConfigPathPattern.MatchString(config.Paths.MultiVideoModel) || len(config.Paths.MultiVideoModel) > 96:
		return nil, errors.New("红果远程配置接口路径无效")
	case !hongguoConfigVersionCode.MatchString(config.Playback.VersionCode),
		!hongguoConfigVersionName.MatchString(config.Playback.VersionName),
		!hongguoConfigUserAgentRunes.MatchString(config.Playback.UserAgent):
		return nil, errors.New("红果远程配置播放参数无效")
	}
	seen := map[int]bool{}
	for _, id := range config.Strategies {
		for index, strategy := range hongguoPlaybackStrategies {
			if strategy.id == id && !seen[index] {
				seen[index] = true
				config.strategyIndexes = append(config.strategyIndexes, index)
			}
		}
	}
	if len(config.strategyIndexes) == 0 {
		return nil, errors.New("红果远程配置没有可用的取流方式")
	}
	return &config, nil
}

var hongguoBundledConfig = sync.OnceValue(func() *hongguoRemoteConfig {
	config, err := parseHongguoRemoteConfig(hongguoBundledRemoteConfig)
	if err != nil {
		panic("内置红果配置无效: " + err.Error())
	}
	return config
})

type hongguoRemoteConfigCache struct {
	FetchedAt time.Time       `json:"fetchedAt"`
	Source    string          `json:"source"`
	Config    json.RawMessage `json:"config"`
}

type hongguoConfigState struct {
	mu         sync.Mutex
	loaded     bool
	current    *hongguoRemoteConfig
	source     string
	fetchedAt  time.Time
	refreshing bool
	lastTry    time.Time
}

// hongguoConfig 返回当前生效的配置：先用内置配置和本地缓存中 revision 较新的一份，
// 过期时在后台刷新，不阻塞调用方。
func (downloader *Downloader) hongguoConfig() *hongguoRemoteConfig {
	state := &downloader.hongguoConfigState
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.loaded {
		state.loaded = true
		state.current, state.source = hongguoBundledConfig(), "内置"
		if cached, fetchedAt, source, ok := downloader.loadHongguoConfigCache(); ok && cached.Revision >= state.current.Revision {
			state.current, state.fetchedAt, state.source = cached, fetchedAt, source
		}
	}
	urls := downloader.cfg.hongguoConfigURLs
	refresh := time.Duration(state.current.RefreshHours) * time.Hour
	if len(urls) > 0 && !state.refreshing && time.Since(state.fetchedAt) >= refresh && time.Since(state.lastTry) >= 10*time.Minute {
		state.refreshing, state.lastTry = true, time.Now()
		go downloader.refreshHongguoConfig(urls)
	}
	return state.current
}

func (downloader *Downloader) refreshHongguoConfig(urls []string) {
	config, raw, source, err := downloader.fetchHongguoRemoteConfig(context.Background(), urls)
	state := &downloader.hongguoConfigState
	state.mu.Lock()
	defer state.mu.Unlock()
	state.refreshing = false
	if err != nil {
		return
	}
	state.fetchedAt = time.Now()
	if config.Revision < state.current.Revision {
		return
	}
	state.current, state.source = config, source
	downloader.saveHongguoConfigCache(hongguoRemoteConfigCache{FetchedAt: state.fetchedAt, Source: source, Config: raw})
}

func (downloader *Downloader) fetchHongguoRemoteConfig(ctx context.Context, urls []string) (*hongguoRemoteConfig, []byte, string, error) {
	var lastErr error
	for _, address := range urls {
		raw, err := downloader.fetchHongguoRemoteConfigFrom(ctx, address)
		if err == nil {
			var config *hongguoRemoteConfig
			if config, err = parseHongguoRemoteConfig(raw); err == nil {
				return config, raw, address, nil
			}
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("没有配置红果远程配置地址")
	}
	return nil, nil, "", lastErr
}

func (downloader *Downloader) fetchHongguoRemoteConfigFrom(ctx context.Context, address string) ([]byte, error) {
	if !hongguoRemoteConfigURLAllowed(address) {
		return nil, errors.New("红果远程配置地址无效")
	}
	ctx, cancel := context.WithTimeout(ctx, hongguoRemoteConfigTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := downloader.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("红果远程配置 HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, hongguoRemoteConfigMaxBytes+1))
	if err != nil {
		return nil, err
	}
	return bytes.TrimSpace(raw), nil
}

// hongguoRemoteConfigURLAllowed 只接受 https；http 仅限本机地址，供开发调试。
func hongguoRemoteConfigURLAllowed(address string) bool {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || parsed.User != nil || len(address) > 512 {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	host := parsed.Hostname()
	return parsed.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1")
}

func hongguoRemoteConfigURLs() []string {
	if override := strings.TrimSpace(os.Getenv(hongguoRemoteConfigURLsEnv)); override != "" {
		var urls []string
		for _, address := range strings.Split(override, ",") {
			if address = strings.TrimSpace(address); hongguoRemoteConfigURLAllowed(address) {
				urls = append(urls, address)
			}
		}
		return urls
	}
	return append([]string(nil), hongguoDefaultRemoteConfigURLs...)
}

func (downloader *Downloader) loadHongguoConfigCache() (*hongguoRemoteConfig, time.Time, string, bool) {
	if downloader.cfg.dataDir == "" {
		return nil, time.Time{}, "", false
	}
	raw, err := os.ReadFile(filepath.Join(downloader.cfg.dataDir, hongguoRemoteConfigCacheFile))
	if err != nil || len(raw) > 4*hongguoRemoteConfigMaxBytes {
		return nil, time.Time{}, "", false
	}
	var cache hongguoRemoteConfigCache
	if json.Unmarshal(raw, &cache) != nil {
		return nil, time.Time{}, "", false
	}
	config, err := parseHongguoRemoteConfig(cache.Config)
	if err != nil {
		return nil, time.Time{}, "", false
	}
	if cache.FetchedAt.After(time.Now()) {
		cache.FetchedAt = time.Time{}
	}
	return config, cache.FetchedAt, "缓存:" + cache.Source, true
}

func (downloader *Downloader) saveHongguoConfigCache(cache hongguoRemoteConfigCache) {
	if raw, err := json.Marshal(cache); err == nil {
		_ = downloader.writeHongguoStateFile(hongguoRemoteConfigCacheFile, raw)
	}
}

func (downloader *Downloader) writeHongguoStateFile(name string, raw []byte) error {
	if downloader.cfg.dataDir == "" {
		return nil
	}
	path := filepath.Join(downloader.cfg.dataDir, name)
	temp := path + ".tmp"
	if err := os.WriteFile(temp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

type hongguoSavedDevice struct {
	DeviceID   string    `json:"deviceId"`
	InstallID  string    `json:"installId"`
	CDID       string    `json:"cdid"`
	Strategy   string    `json:"strategy"`
	VerifiedAt time.Time `json:"verifiedAt"`
}

// loadHongguoSavedDevice 读回上次验证可用的设备；超过 deviceTTLHours 或格式不对就丢弃。
func (downloader *Downloader) loadHongguoSavedDevice(config *hongguoRemoteConfig) (*hongguoPlaybackDevice, string, time.Time) {
	if downloader.cfg.dataDir == "" || !config.ReuseDevice || config.DeviceTTLHours == 0 {
		return nil, "", time.Time{}
	}
	raw, err := os.ReadFile(filepath.Join(downloader.cfg.dataDir, hongguoPlaybackDeviceFile))
	if err != nil || len(raw) > 4096 {
		return nil, "", time.Time{}
	}
	var saved hongguoSavedDevice
	if json.Unmarshal(raw, &saved) != nil || !hongguoNumericID.MatchString(saved.DeviceID) || !hongguoNumericID.MatchString(saved.InstallID) {
		return nil, "", time.Time{}
	}
	age := time.Since(saved.VerifiedAt)
	if age < 0 || age > time.Duration(config.DeviceTTLHours)*time.Hour {
		return nil, "", time.Time{}
	}
	return &hongguoPlaybackDevice{DeviceID: saved.DeviceID, Install: saved.InstallID, CDID: saved.CDID}, saved.Strategy, saved.VerifiedAt
}

func (downloader *Downloader) saveHongguoDevice(device hongguoPlaybackDevice, strategy string, config *hongguoRemoteConfig) {
	if !config.ReuseDevice || config.DeviceTTLHours == 0 {
		return
	}
	raw, err := json.Marshal(hongguoSavedDevice{DeviceID: device.DeviceID, InstallID: device.Install, CDID: device.CDID, Strategy: strategy, VerifiedAt: time.Now()})
	if err == nil {
		_ = downloader.writeHongguoStateFile(hongguoPlaybackDeviceFile, raw)
	}
}

func (downloader *Downloader) forgetHongguoDevice() {
	if downloader.cfg.dataDir != "" {
		_ = os.Remove(filepath.Join(downloader.cfg.dataDir, hongguoPlaybackDeviceFile))
	}
}
