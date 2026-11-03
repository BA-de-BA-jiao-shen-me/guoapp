package core

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"math/bits"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func signHongguoPlaybackRequest(request *http.Request, body []byte, now time.Time) {
	khronos := uint32(now.Unix())
	query := request.URL.Query()
	ticket := now.UnixMilli()
	for attempt := 0; attempt < 64; attempt++ {
		query.Set("_rticket", strconv.FormatInt(ticket+int64(attempt), 10))
		request.URL.RawQuery = query.Encode()
		if hongguoSignBranch(request.URL.RawQuery, body, khronos) == hongguoSignValidBranch {
			break
		}
	}
	for key, value := range hongguoPureSign(request.URL.String(), body, khronos) {
		request.Header.Set(key, value)
	}
}

// hongguoSignValidBranch 是本地实现的 X-Medusa/X-Gorgon 唯一能被上游接受的分支。
// 实测（2026-11-03）：分支 0 与分支 1 的签名一律回 HTTP 200 空正文，只有分支 2 返回 video_model。
// 分支由 query/body/时间戳决定，换一个 _rticket 就会变，所以签名前调整 _rticket 直到落在分支 2。
const hongguoSignValidBranch = 2

func hongguoPureSign(rawURL string, body []byte, khronos uint32) map[string]string {
	query := hongguoSignQuery(rawURL)
	gorgonRandom := hongguoRandInt(65536)
	return map[string]string{
		"X-Gorgon":  hongguoXGorgon(query, body, khronos, gorgonRandom),
		"X-Khronos": strconv.FormatUint(uint64(khronos), 10),
		"X-Ladon":   base64.StdEncoding.EncodeToString(hongguoBe32(khronos)),
		"X-Argus":   base64.StdEncoding.EncodeToString(hongguoLe32(khronos)),
		"X-Helios":  hongguoHelios(khronos, hongguoRandInt32()),
		"X-Medusa":  hongguoMedusa(rawURL, body, khronos),
	}
}

func hongguoSignQuery(rawURL string) string {
	if index := strings.IndexByte(rawURL, '?'); index >= 0 {
		return rawURL[index+1:]
	}
	return ""
}

func hongguoSignBranch(query string, body []byte, khronos uint32) int {
	iv := hongguoSignIV(query, body, khronos)
	low := iv & 15
	return int(low - ((low*171)>>9)*3)
}

func hongguoSignIV(query string, body []byte, khronos uint32) uint32 {
	return hongguoGetIV(hongguoGetIV(hongguoGetIV(0x20230928, hongguoSM3Bytes([]byte(query))), hongguoBodyMD5(body)), hongguoLe32(khronos))
}

func hongguoBodyMD5(body []byte) []byte {
	if len(body) == 0 {
		return make([]byte, 16)
	}
	sum := md5.Sum(body)
	return sum[:]
}

func hongguoXGorgon(query string, body []byte, timestamp uint32, random int) string {
	queryHash := md5.Sum([]byte(query))
	input := append([]byte{}, queryHash[:4]...)
	if len(body) > 0 {
		bodyHash := md5.Sum(body)
		input = append(input, bodyHash[:4]...)
	} else {
		input = append(input, 0, 0, 0, 0)
	}
	input = append(input, 0, 0, 0, 0)
	input = append(input, hongguoLe32(67503104)...)
	input = append(input, hongguoBe32(timestamp)...)
	key := []byte{0x4a, 0x40, 0x16, byte(random >> 8), 0x47, 0x6c, 0x01, byte(random)}
	out := hongguoRC4Gorgon(input, key)
	for index := range out {
		value := hongguoRotl8(out[index])
		next := out[0]
		if index+1 < len(out) {
			next = out[index+1]
		}
		out[index] = ^(bits.Reverse8(next^value) ^ 20)
	}
	prefix := []byte{0x84, 0x04, byte(random), byte(random >> 8), 0x40, 0x01}
	return hex.EncodeToString(append(prefix, out...))
}

func hongguoRC4Gorgon(data, key []byte) []byte {
	var box [256]byte
	for index := range box {
		box[index] = byte(index)
	}
	j := 0
	for index := 0; index < 256; index++ {
		j = (j + int(box[index]) + int(key[index%len(key)])) & 255
		box[index] = box[j]
	}
	out := make([]byte, len(data))
	i, y := 0, 0
	for index, current := range data {
		i = (i + 1) & 255
		x := int(box[i])
		y = (y + x) & 255
		box[i] = box[y]
		out[index] = current ^ box[(int(box[y])+int(box[y]))&255]
	}
	return out
}

func hongguoHelios(khronos uint32, random int32) string {
	sum := md5.Sum(append(hongguoLe32(uint32(random)), []byte("8662")...))
	ascii := []byte(hex.EncodeToString(sum[:]))
	words := [4]uint64{}
	for index := range words {
		words[index] = binary.LittleEndian.Uint64(ascii[index*8:])
	}
	queue := []uint64{words[2], words[3]}
	b0, b8 := words[0], words[1]
	table := make([]uint64, 35)
	table[0] = words[0]
	for index := uint64(0); index < 34; index++ {
		x8 := bits.RotateLeft64(b8, -8) + b0
		x8 ^= index
		queue = append(queue, x8)
		x8 ^= bits.RotateLeft64(b0, -61)
		table[index+1] = x8
		b0 = x8
		b8 = queue[0]
		queue = queue[1:]
	}
	text := strconv.FormatUint(uint64(khronos), 10) + "-1588093228-8662"
	padded := hongguoBlockPad([]byte(text))
	var output []byte
	for at := 0; at < len(padded); at += 16 {
		a := binary.LittleEndian.Uint64(padded[at:])
		b := binary.LittleEndian.Uint64(padded[at+8:])
		for index := 0; index < 34; index++ {
			b = table[index] ^ (a + bits.RotateLeft64(b, -8))
			a = b ^ bits.RotateLeft64(a, -61)
		}
		block := make([]byte, 16)
		binary.LittleEndian.PutUint64(block, a)
		binary.LittleEndian.PutUint64(block[8:], b)
		output = append(output, block...)
	}
	return base64.StdEncoding.EncodeToString(append(hongguoLe32(uint32(random)), output...))
}

func hongguoMedusa(rawURL string, body []byte, khronos uint32) string {
	query := hongguoSignQuery(rawURL)
	querySM3 := hongguoSM3Bytes([]byte(query))
	bodyMD5 := hongguoBodyMD5(body)
	mixed := hongguoHashF13(querySM3, bodyMD5, hongguoLe32(khronos), khronos)
	messageRandom := hongguoRandBytes(4)
	messageRandomU := binary.LittleEndian.Uint32(messageRandom)
	env := hongguoProtoJoin(
		hongguoProtoSint(1, int64(100+hongguoRandInt(21))),
		hongguoProtoSint(2, 146331399),
		hongguoProtoSint(3, 146331396),
		hongguoProtoSint(5, 7),
		hongguoProtoString(6, "v04.06.04.03-bugfix"),
		hongguoProtoSint(7, int64(10001+hongguoRandInt(2000))),
		hongguoProtoMessage(12, hongguoDeviceProto(hongguoSignParam(query, "device_id", "0"))),
		hongguoProtoMessage(13, hongguoProtoJoin(
			hongguoProtoSint(1, time.Now().Unix()),
			hongguoProtoSint(2, -2),
			hongguoProtoSint(4, 200),
		)),
		hongguoProtoString(14, hongguoSignParam(query, "version_name", "7.1.3.32")),
	)
	queryHash := hongguoSM3Bytes(append(append([]byte(query), bodyMD5...), []byte("none")...))
	nested := hongguoProtoJoin(
		hongguoProtoSint(1, 111),
		hongguoProtoSint(2, 10),
		hongguoProtoSint(3, 694367),
		hongguoProtoSint(5, 586952199),
	)
	message := hongguoProtoJoin(
		hongguoProtoBytes(1, hongguoMedusaMagic16),
		hongguoProtoSint(2, 3),
		hongguoProtoSint(3, int64(messageRandomU)),
		hongguoProtoString(4, "8662"),
		hongguoProtoString(5, hongguoSignParam(query, "device_id", "0")),
		hongguoProtoString(6, "1588093228"),
		hongguoProtoString(7, hongguoSignParam(query, "version_name", "7.1.3.32")),
		hongguoProtoString(8, "v04.06.04-ml-android"),
		hongguoProtoSint(9, 67503104),
		hongguoProtoBytes(10, hongguoMedusaMagic8),
		hongguoProtoSint(12, int64(khronos)),
		hongguoProtoBytes(13, mixed),
		hongguoProtoBytes(14, querySM3[:6]),
		hongguoProtoMessage(15, nested),
		hongguoProtoString(16, hongguoProcessNonce),
		hongguoProtoSint(17, int64(khronos)),
		hongguoProtoBytes(19, queryHash),
		hongguoProtoString(20, "none"),
		hongguoProtoSint(21, 312),
		hongguoProtoMessage(23, env),
		hongguoProtoString(24, `{"cmr":16777216,"cmr2":16777216,"un_h":1879194040,"vpn":0,"kd":0,"fkd":3672518972,"pd":-1872573247,"dyn":"","do":0,"tk":true}`),
	)
	random := hongguoRandInt32()
	hash, seed := hongguoKeyHash(hongguoMedusaSignKey, random)
	transformed := hongguoXMXOR(message, hash)
	transformed = append(append([]byte{}, hongguoMedusaMagic8...), transformed...)
	for left, right := 0, len(transformed)-1; left < right; left, right = left+1, right-1 {
		transformed[left], transformed[right] = transformed[right], transformed[left]
	}
	for index := range transformed {
		transformed[index] ^= seed[(^index)&3]
	}
	check := ((uint32(querySM3[0]) & 63) << 14) | 0x18000001 | ((uint32(mixed[0]) & 63) << 8)
	packed := append([]byte{0x35}, hongguoLe32(uint32(random))...)
	packed = append(packed, hongguoLe32(check)...)
	packed = append(packed, transformed...)
	packed = append(packed, byte(uint32(random)>>16), byte(uint32(random)>>24))
	encrypted := newHongguoAesV3(hongguoMedusaAESKey, khronos).encrypt(packed, hongguoMedusaAESIV)
	version := []byte{0x03, 0, 0, 0, 0xf7, 0xe8, 0x5f, 0xfa, 0xd7, 0xd7, 0xdc, 0x3b, 0xd6, 0x2a, 0xc8, 0x70, 0x57, 0xcf, 0x61, 0x18}
	prefix := make([]byte, 20)
	for index := 0; index < 20; index += 4 {
		binary.LittleEndian.PutUint32(prefix[index:], binary.LittleEndian.Uint32(version[index:])^khronos)
	}
	prefix = append(prefix, byte(random), byte(uint32(random)>>8), 0, 1)
	return base64.StdEncoding.EncodeToString(append(prefix, encrypted...))
}
