package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var errHongguoDeviceRegister = errors.New("红果播放设备注册失败")

const hongguoRegisterURL = "https://log5-applog-lf.fqnovel.com/service/2/device_register/?tt_data=a&cronet_version=8d40f833_2026-03-03&ttnet_version=4.2.243.31-douyin&use_store_region_cookie=1"

const hongguoPlaybackUserAgent = "com.phoenix.read/71532 (Linux; U; Android 9; SM-N9860; Build/PQ3A.190705.10241111;tt-ok/3.12.13.20)"

var hongguoRegisterSalt = []byte{
	0x4d, 0xd4, 0xc2, 0xe6, 0xb8, 0x31, 0x62, 0x09, 0x0e, 0x52, 0xb3, 0xc7, 0xa6, 0x73, 0x3b, 0xa4,
	0x1c, 0xb2, 0x46, 0x2b, 0x82, 0x9a, 0xb5, 0x8a, 0x19, 0x6b, 0x39, 0xdb, 0x57, 0x17, 0x75, 0x24,
	0xf4, 0x9b, 0xaf, 0x7f, 0x08, 0xe8, 0xd6, 0x8d, 0x26, 0xa7, 0x2e, 0x37, 0xc1, 0xa9, 0x5a, 0x2f,
	0x1f, 0x05, 0xa5, 0x18, 0x92, 0xae, 0xf2, 0x94, 0x97, 0x32, 0xb6, 0x2a, 0x38, 0xaa, 0xdd, 0x58,
}

type hongguoPlaybackDevice struct {
	DeviceID string
	Install  string
	CDID     string
}

func (downloader *Downloader) hongguoPlaybackDevice(ctx context.Context) (hongguoPlaybackDevice, error) {
	client := downloader.hongguoClient()
	client.mu.Lock()
	cached := client.playbackDevice
	client.mu.Unlock()
	if cached.DeviceID != "" && cached.Install != "" {
		return cached, nil
	}
	device, err := downloader.registerHongguoPlaybackDevice(ctx)
	if err != nil {
		return hongguoPlaybackDevice{}, fmt.Errorf("%w：%v", errHongguoDeviceRegister, err)
	}
	client.mu.Lock()
	if client.playbackDevice.DeviceID == "" {
		client.playbackDevice = device
	} else {
		device = client.playbackDevice
	}
	client.mu.Unlock()
	return device, nil
}

func (downloader *Downloader) registerHongguoPlaybackDevice(ctx context.Context) (hongguoPlaybackDevice, error) {
	now := time.Now().UnixMilli()
	cdid, err := hongguoRandomUUID()
	if err != nil {
		return hongguoPlaybackDevice{}, err
	}
	body, err := hongguoRegisterBody(now, cdid)
	if err != nil {
		return hongguoPlaybackDevice{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, hongguoRegisterURL, bytes.NewReader(body))
	if err != nil {
		return hongguoPlaybackDevice{}, err
	}
	sum := md5.Sum(body)
	request.Header.Set("User-Agent", "com.phoenix.read/72232 (Linux; U; Android 16; zh_CN; 25053RT47C; Build/BP2A.250605.031.A3)")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/octet-stream;tt-data=a")
	request.Header.Set("log-encode-type", "gzip")
	request.Header.Set("x-ss-dp", "8662")
	request.Header.Set("x-ss-req-ticket", strconv.FormatInt(now, 10))
	request.Header.Set("x-vc-bdturing-sdk-version", "4.0.3.cn")
	request.Header.Set("lc", "101")
	request.Header.Set("sdk-version", "2")
	request.Header.Set("passport-sdk-version", "5051452")
	request.Header.Set("x-neptune", "-8|50:51:59:30:40:47:49:39")
	request.Header.Set("x-ss-stub", strings.ToUpper(hex.EncodeToString(sum[:])))
	response, err := downloader.doCatalogRequestWithTimeout(request, 12*time.Second)
	if err != nil {
		return hongguoPlaybackDevice{}, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return hongguoPlaybackDevice{}, err
	}
	if response.StatusCode != http.StatusOK || len(payload) == 0 {
		return hongguoPlaybackDevice{}, errors.New("红果播放设备注册没有返回身份")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var decoded map[string]any
	if decoder.Decode(&decoded) != nil || decoded == nil {
		return hongguoPlaybackDevice{}, errors.New("红果播放设备注册返回格式异常")
	}
	if code := mapString(decoded, "code", "Code"); code != "" && code != "0" {
		return hongguoPlaybackDevice{}, errors.New("红果播放设备注册被拒绝")
	}
	deviceID := hongguoResponseID(decoded, "device_id")
	installID := hongguoResponseID(decoded, "install_id")
	if deviceID == "" || installID == "" {
		return hongguoPlaybackDevice{}, errors.New("红果播放设备注册未返回设备号")
	}
	return hongguoPlaybackDevice{DeviceID: deviceID, Install: installID, CDID: cdid}, nil
}

func hongguoResponseID(payload map[string]any, name string) string {
	if text := mapString(payload, name+"_str"); hongguoNumericID.MatchString(text) && strings.ContainsAny(text, "123456789") {
		return text
	}
	text := mapString(payload, name)
	if hongguoNumericID.MatchString(text) && strings.ContainsAny(text, "123456789") {
		return text
	}
	return ""
}

func hongguoRegisterBody(now int64, cdid string) ([]byte, error) {
	openUDID := make([]byte, 8)
	if _, err := rand.Read(openUDID); err != nil {
		return nil, err
	}
	clientID, err := hongguoRandomUUID()
	if err != nil {
		return nil, err
	}
	requestID, err := hongguoRandomUUID()
	if err != nil {
		return nil, err
	}
	header := map[string]any{
		"display_name": "红果免费短剧", "update_version_code": 73932, "manifest_version_code": 73932,
		"app_version_minor": "7.3.9.32", "aid": 8662, "channel": "update_64", "package": "com.phoenix.read",
		"app_version": "7.3.9.32", "version_code": 73932, "sdk_version": "3.7.0-rc.34-fanqie-xiaoshuo",
		"sdk_target_version": 29, "git_hash": "3a88f4e", "os": "Android", "os_version": "16", "os_api": 36,
		"device_model": "25053RT47C", "device_brand": "Redmi", "device_manufacturer": "Redmi",
		"cpu_abi": "arm64-v8a", "release_build": "b5e187f", "density_dpi": 520, "display_density": "mdpi",
		"resolution": "1280x2772", "language": "zh", "timezone": 8, "access": "wifi", "not_request_sender": 0,
		"carrier": "CHINA MOBILE", "mcc_mnc": "46000", "rom": "eng.root.20231116.160340",
		"rom_version": "W528JS release-keys", "cdid": cdid,
		"sig_hash": "56a962410c494bbaf0b58dba20cae56f", "openudid": hex.EncodeToString(openUDID),
		"clientudid": clientID, "ipv6_list": []any{}, "region": "CN", "tz_name": "Asia/Shanghai",
		"tz_offset": 28800, "sim_region": "cn", "sim_serial_number": []any{}, "oaid_may_support": false,
		"req_id": requestID, "device_platform": "android",
		"custom":                map[string]any{"account_region_src": "fallback", "account_region": "cn", "host_bit": 64, "dragon_device_type": "phone"},
		"pre_installed_channel": "", "apk_first_install_time": now, "is_system_app": 0, "sdk_flavor": "china", "guest_mode": 0,
	}
	raw, err := json.Marshal(map[string]any{"magic_tag": "ss_app_log", "header": header, "_gen_time": now})
	if err != nil {
		return nil, err
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err = writer.Write(raw); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		return nil, err
	}
	first := sha512.Sum512(random)
	derived := sha512.Sum512(append(first[:], hongguoRegisterSalt...))
	block, err := aes.NewCipher(derived[:16])
	if err != nil {
		return nil, err
	}
	digest := sha512.Sum512(compressed.Bytes())
	plain := pkcs7Pad(append(digest[:], compressed.Bytes()...), aes.BlockSize)
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, derived[16:32]).CryptBlocks(encrypted, plain)
	return append(append([]byte{0x74, 0x63, 0x05, 0x10, 0x00, 0x00}, random...), encrypted...), nil
}

func (downloader *Downloader) resolveHongguoRegisteredMedia(ctx context.Context, videoID string) (providerMedia, error) {
	device, err := downloader.hongguoPlaybackDevice(ctx)
	if err != nil {
		return providerMedia{}, err
	}
	var lastErr error
	for _, contentType := range []string{"1004", "1"} {
		result, err := downloader.hongguoSignedRegisteredMedia(ctx, device, videoID, contentType)
		if err != nil && ctx.Err() == nil {
			result, err = downloader.hongguoUnsignedMedia(ctx, device, videoID, contentType)
		}
		if err != nil {
			lastErr = err
			continue
		}
		model, err := hongguoVideoModel(result, videoID)
		if err != nil {
			lastErr = err
			continue
		}
		if followed, ferr := downloader.hongguoFallbackModel(ctx, result, model, videoID); ferr == nil && len(followed) > 0 {
			model = followed
		}
		if len(model) == 0 {
			lastErr = errors.New("红果注册设备未返回正片")
			continue
		}
		media, err := selectHongguoAppMedia(model)
		if err != nil {
			lastErr = err
			continue
		}
		if hongguoPublicVideoPattern.MatchString(media.URL) && media.Duration < 30*time.Second {
			lastErr = errors.New("红果注册设备只返回了试看")
			continue
		}
		return media, nil
	}
	if lastErr == nil {
		lastErr = errors.New("红果注册设备未返回正片")
	}
	return providerMedia{}, lastErr
}

// hongguoSignedRegisteredMedia 用注册得到的真实设备号，并带上六个播放签名头请求取流。
// 2026-11 起上游对不带签名的取流请求回 HTTP 200、空正文（0.2.129 的无签名方案失效）；
// 随机设备号带签名同样是空正文，只有“注册设备 + 签名”能拿到 video_model。
func (downloader *Downloader) hongguoSignedRegisteredMedia(ctx context.Context, device hongguoPlaybackDevice, videoID, contentType string) (map[string]any, error) {
	payload := map[string]any{
		"biz_param": map[string]any{
			"detail_page_version": 0, "device_level": 3, "disable_digg_stat": false,
			"need_all_video_definition": true, "need_mp4_align": false, "use_os_player": false,
			"use_server_dns": false, "video_platform": 1024,
		},
		"mixed_video_id_map": map[string]any{contentType: []string{videoID}},
	}
	signed := context.WithValue(ctx, hongguoPlaybackSignKey{}, true)
	return downloader.hongguoAppRequestOrdered(signed, http.MethodPost, hongguoMediaPath, url.Values{
		"device_id": {device.DeviceID}, "iid": {device.Install},
	}, payload, false)
}

func (downloader *Downloader) hongguoUnsignedMedia(ctx context.Context, device hongguoPlaybackDevice, videoID, contentType string) (map[string]any, error) {
	body, err := json.Marshal(map[string]any{
		"biz_param": map[string]any{
			"detail_page_version": 0, "device_level": 3, "disable_digg_stat": false,
			"need_all_video_definition": true, "need_mp4_align": false, "use_os_player": false,
			"use_server_dns": false, "video_platform": 1024,
		},
		"mixed_video_id_map": map[string]any{contentType: []string{videoID}},
	})
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, base := range hongguoAppBases(false) {
		endpoint := strings.TrimRight(base, "/") + hongguoMediaPath + "?" + hongguoPlaybackQuery(device)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		request.Header.Set("User-Agent", hongguoPlaybackUserAgent)
		request.Header.Set("Accept", "application/json; charset=utf-8,application/x-protobuf")
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		response, err := downloader.doCatalogRequestWithTimeout(request, 15*time.Second)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, err
			}
			continue
		}
		content, readErr := io.ReadAll(io.LimitReader(response.Body, providerMaxBodyBytes+1))
		response.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if response.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("红果注册设备取流 HTTP %d", response.StatusCode)
			if response.StatusCode >= 400 && response.StatusCode < 500 {
				return nil, lastErr
			}
			continue
		}
		if len(content) == 0 || len(content) > providerMaxBodyBytes {
			lastErr = errors.New("红果注册设备取流返回空内容")
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.UseNumber()
		var result map[string]any
		if decoder.Decode(&result) != nil || result == nil {
			lastErr = errors.New("红果注册设备取流返回格式异常")
			continue
		}
		code := firstNonEmpty(mapString(result, "code", "Code", "status_code"), mapString(nestedMap(result, "BaseResp"), "StatusCode"))
		if code != "" && code != "0" {
			lastErr = fmt.Errorf("红果注册设备取流暂不可用（%s）", truncate(code, 20))
			continue
		}
		return result, nil
	}
	if lastErr == nil {
		lastErr = errors.New("红果注册设备取流返回空内容")
	}
	return nil, lastErr
}

func hongguoPlaybackQuery(device hongguoPlaybackDevice) string {
	pairs := [][2]string{
		{"ac", "wifi"}, {"channel", "update_64"}, {"aid", "8662"}, {"app_name", "novelread"},
		{"version_code", "71332"}, {"version_name", "7.1.3.32"}, {"device_platform", "android"},
		{"os", "android"}, {"ssmix", "a"}, {"device_type", "25053RT47C"}, {"device_brand", "Redmi"},
		{"language", "zh"}, {"os_api", "36"}, {"os_version", "16"}, {"manifest_version_code", "71332"},
		{"resolution", "1280*2772"}, {"dpi", "520"}, {"update_version_code", "71332"},
		{"host_abi", "arm64-v8a"}, {"dragon_device_type", "phone"}, {"pv_player", "71332"},
		{"compliance_status", "0"}, {"need_personal_recommend", "1"}, {"player_so_load", "1"},
		{"is_android_pad_screen", "0"}, {"device_id", device.DeviceID}, {"iid", device.Install},
		{"_rticket", strconv.FormatInt(time.Now().UnixMilli(), 10)},
	}
	var query strings.Builder
	for index, pair := range pairs {
		if index > 0 {
			query.WriteByte('&')
		}
		query.WriteString(url.QueryEscape(pair[0]))
		query.WriteByte('=')
		query.WriteString(url.QueryEscape(pair[1]))
	}
	return query.String()
}

func hongguoRandomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:]), nil
}
