package core

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (downloader *Downloader) resolveHongguoAppMedia(ctx context.Context, videoID string) (providerMedia, error) {
	if !hongguoNumericID.MatchString(videoID) {
		return providerMedia{}, errors.New("红果视频 ID 无效")
	}
	if media, err := downloader.resolveHongguoRegisteredMedia(ctx, videoID); err == nil || !errors.Is(err, errHongguoDeviceRegister) {
		return media, err
	}
	deviceID, err := newHongguoPlaybackDeviceID()
	if err != nil {
		return providerMedia{}, err
	}
	payload := hongguoVideoModelPayload(videoID)
	ctx = context.WithValue(ctx, hongguoPlaybackSignKey{}, true)
	result, err := downloader.hongguoAppRequestOrdered(ctx, http.MethodPost, hongguoVideoModelPath, url.Values{
		"device_id": {deviceID}, "iid": {deviceID},
	}, payload, false)
	if err != nil {
		return providerMedia{}, err
	}
	model, err := hongguoVideoModel(result, videoID)
	if err != nil {
		return providerMedia{}, err
	}
	if followed, ferr := downloader.hongguoFallbackModel(ctx, result, model, videoID); ferr == nil && len(followed) > 0 {
		model = followed
	}
	return selectHongguoAppMedia(model)
}

func hongguoVideoModel(result map[string]any, videoID string) (map[string]any, error) {
	data := nestedMap(result, "data")
	model, err := decodeHongguoVideoModel(data["video_model"])
	if err != nil || len(model) > 0 {
		return model, err
	}
	row := nestedMap(data, videoID)
	model, err = decodeHongguoVideoModel(row["video_model"])
	if err != nil || len(model) > 0 {
		return model, err
	}
	if len(row) > 0 {
		return row, nil
	}
	return nil, nil
}

func decodeHongguoVideoModel(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	if model, ok := value.(map[string]any); ok {
		return model, nil
	}
	encoded, ok := value.(string)
	if !ok || strings.TrimSpace(encoded) == "" {
		return nil, errors.New("红果 App 播放信息格式异常")
	}
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.UseNumber()
	var model map[string]any
	if err := decoder.Decode(&model); err != nil {
		return nil, errors.New("红果 App 播放信息格式异常")
	}
	return model, nil
}

func selectHongguoAppMedia(model map[string]any) (providerMedia, error) {
	variants := anyList(model["video_list"])
	if rows, ok := model["video_list"].(map[string]any); ok && len(variants) == 0 {
		keys := make([]string, 0, len(rows))
		for key := range rows {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			variants = append(variants, rows[key])
		}
	}
	duration, _ := strconv.ParseFloat(mapString(model, "video_duration", "duration"), 64)
	var keyErr error
	type scoredMedia struct {
		media providerMedia
		score int
	}
	var choices []scoredMedia
	for _, row := range variants {
		variant, _ := row.(map[string]any)
		meta := nestedMap(variant, "video_meta")
		encryption := nestedMap(variant, "encrypt_info")
		codec := strings.ToLower(hongguoField(variant, meta, "codec_type"))
		if codec == "bytevc2" || strings.Contains(strings.ToLower(mapString(variant, "gear_des_key")), "bytevc2") {
			continue
		}
		addresses := hongguoMediaAddresses(variant, hongguoKeySeed(model, variant, encryption))
		if len(addresses) == 0 {
			continue
		}
		media := providerMedia{Referer: "https://novel.snssdk.com/", Duration: time.Duration(duration * float64(time.Second))}
		spade := hongguoField(variant, encryption, "spade_a")
		if spade != "" || variant["encrypt"] == true || encryption["encrypt"] == true || hongguoField(variant, encryption, "encryption_method") == "cenc-aes-ctr" {
			var err error
			media.CENCKey, err = hongguoContentKey(spade)
			if err != nil {
				keyErr = err
				continue
			}
		}
		height, _ := strconv.Atoi(hongguoField(variant, meta, "vheight"))
		if definition, err := strconv.Atoi(hongguoQualityNumber.FindString(hongguoField(variant, meta, "definition"))); err == nil && definition > 0 {
			height = definition
		} else if width, _ := strconv.Atoi(hongguoField(variant, meta, "vwidth")); width > 0 && (height == 0 || width < height) {
			height = width
		}
		media.Quality = height
		quality := height * 10
		if codec == "h264" || codec == "avc1" {
			quality++
		}
		for _, address := range addresses {
			media.URL = address
			choices = append(choices, scoredMedia{media: media, score: quality})
		}
	}
	if len(choices) > 0 {
		sort.SliceStable(choices, func(i, j int) bool { return choices[i].score > choices[j].score })
		selected := choices[0].media
		for _, choice := range choices {
			selected.Variants = append(selected.Variants, choice.media)
		}
		return selected, nil
	}
	if keyErr != nil {
		return providerMedia{}, fmt.Errorf("红果 App 媒体密钥不可用: %w", keyErr)
	}
	return providerMedia{}, errors.New("红果 App 未返回兼容的媒体，已跳过不支持的编码")
}

func hongguoField(primary, fallback map[string]any, key string) string {
	if value := mapString(primary, key); value != "" {
		return value
	}
	return mapString(fallback, key)
}

func hongguoKeySeed(model, variant, encryption map[string]any) []byte {
	encoded := firstNonEmpty(mapString(variant, "key_seed"), mapString(encryption, "key_seed"), mapString(model, "key_seed"))
	if encoded == "" {
		return nil
	}
	seed, err := decodeHongguoBase64(encoded)
	if err != nil || len(seed) == 0 || len(seed) > 256 {
		return nil
	}
	return seed
}

func hongguoFallbackAddress(result, model map[string]any) string {
	for _, source := range []map[string]any{model, nestedMap(result, "data"), result} {
		if address := mapString(source, "fallback_api"); isProviderHTTPMediaURL(address) {
			return address
		}
	}
	return ""
}

func (downloader *Downloader) hongguoFallbackModel(ctx context.Context, result, model map[string]any, videoID string) (map[string]any, error) {
	address := hongguoFallbackAddress(result, model)
	if address == "" {
		return nil, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := downloader.doCatalogRequestWithTimeout(request, 20*time.Second)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, providerMaxBodyBytes+1))
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || len(body) == 0 || len(body) > providerMaxBodyBytes {
		return nil, errors.New("红果档位表没有返回")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var followed map[string]any
	if decoder.Decode(&followed) != nil || followed == nil {
		return nil, errors.New("红果档位表格式异常")
	}
	if _, ok := followed["video_list"]; ok {
		return followed, nil
	}
	return hongguoVideoModel(followed, videoID)
}

func hongguoMediaAddresses(info map[string]any, keySeed []byte) []string {
	var addresses []string
	seen := map[string]bool{}
	var add func(any)
	add = func(value any) {
		switch value := value.(type) {
		case string:
			original := strings.TrimSpace(value)
			if len(original) > 8192 {
				return
			}
			address := original
			if !isProviderHTTPMediaURL(address) {
				decoded, err := decodeHongguoBase64(address)
				if err == nil {
					address = strings.TrimSpace(string(decoded))
				}
				if !isProviderHTTPMediaURL(address) && len(keySeed) > 0 {
					plain, derr := decryptHongguoSeededURL(original, keySeed)
					if derr != nil {
						return
					}
					address = plain
				}
			}
			if isProviderHTTPMediaURL(address) && !seen[address] {
				seen[address] = true
				addresses = append(addresses, address)
			}
		case []any:
			for _, item := range value {
				add(item)
			}
		}
	}
	for _, key := range []string{"main_url", "backup_url", "backup_url_1", "backup_url_2", "backup_urls", "url_list"} {
		add(info[key])
	}
	return addresses
}

var hongguoSeedURLConstant = []byte{
	0x4d, 0xd4, 0xc2, 0xe6, 0xb8, 0x31, 0x62, 0x09, 0x0e, 0x52, 0xb3, 0xc7, 0xa6, 0x73, 0x3b, 0xa4,
	0x1c, 0xb2, 0x46, 0x2b, 0x82, 0x9a, 0xb5, 0x8a, 0x19, 0x6b, 0x39, 0xdb, 0x57, 0x17, 0x75, 0x24,
	0xf4, 0x9b, 0xaf, 0x7f, 0x08, 0xe8, 0xd6, 0x8d, 0x26, 0xa7, 0x2e, 0x37, 0xc1, 0xa9, 0x5a, 0x2f,
	0x1f, 0x05, 0xa5, 0x18, 0x92, 0xae, 0xf2, 0x94, 0x97, 0x32, 0xb6, 0x2a, 0x38, 0xaa, 0xdd, 0x58,
}

func decryptHongguoSeededURL(encoded string, keySeed []byte) (string, error) {
	raw, err := decodeHongguoBase64(encoded)
	if err != nil || len(raw) < 20 || raw[0] != 0xA8 || raw[2] != 1 || raw[3] != 0 {
		return "", errors.New("红果媒体地址密钥无效")
	}
	cipherText := raw[4 : 4+(len(raw)-4)/aes.BlockSize*aes.BlockSize]
	if len(cipherText) == 0 {
		return "", errors.New("红果媒体地址密钥无效")
	}
	sum := sha512.Sum512(keySeed)
	material := sha512.Sum512(append(sum[:], hongguoSeedURLConstant...))
	block, err := aes.NewCipher(material[:aes.BlockSize])
	if err != nil {
		return "", err
	}
	plain := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, material[aes.BlockSize:aes.BlockSize*2]).CryptBlocks(plain, cipherText)
	plain, err = pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return "", errors.New("红果媒体地址解密失败")
	}
	address := strings.TrimRight(string(plain), "\x00")
	if !isProviderHTTPMediaURL(address) {
		return "", errors.New("红果媒体地址解密失败")
	}
	return address, nil
}
