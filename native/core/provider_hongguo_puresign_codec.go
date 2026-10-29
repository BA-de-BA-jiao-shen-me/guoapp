package core

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"math/bits"
	"time"
)

var hongguoProcessNonce = func() string {
	var raw [18]byte
	_, _ = rand.Read(raw[:])
	return base64.RawURLEncoding.EncodeToString(raw[:])
}()

var hongguoProcessStarted = time.Now().UnixMilli()

var hongguoMedusaSignKey = []byte{
	0x8e, 0xbd, 0xfa, 0x38, 0x06, 0xec, 0xc5, 0xce, 0xe7, 0x94, 0x23, 0xe6, 0x02, 0x9e, 0xd8, 0x25,
	0x40, 0xbc, 0x22, 0x18, 0xbb, 0x7e, 0xae, 0xf7, 0x1c, 0xb6, 0x91, 0xf7, 0xaa, 0x8a, 0xa2, 0xf5,
}
var hongguoMedusaAESKey = []byte{0xf1, 0x59, 0x33, 0x76, 0x76, 0x6e, 0xa9, 0x8d, 0x34, 0xf3, 0x1b, 0x05, 0x7a, 0x9d, 0x5b, 0xe4}
var hongguoMedusaAESIV = []byte{0x1f, 0xe1, 0x09, 0xa4, 0x12, 0x52, 0x83, 0xf4, 0x18, 0xde, 0x9e, 0x05, 0x1a, 0x96, 0x9e, 0x12}
var hongguoMedusaMagic16 = []byte{0xf7, 0xe8, 0x5f, 0xfa, 0xd7, 0xd7, 0xdc, 0x3b, 0xd6, 0x2a, 0xc8, 0x70, 0x57, 0xcf, 0x61, 0x18}
var hongguoMedusaMagic8 = []byte{0x40, 0x01, 0, 0, 0, 0, 0, 0}

func hongguoLe32(value uint32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, value)
	return out
}

func hongguoBe32(value uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, value)
	return out
}

func hongguoRotl8(value byte) byte {
	return value<<4 | value>>4
}

func hongguoRotl8Three(value byte) byte {
	return value<<3 | value>>5
}

func hongguoRotl32(value uint32, n int) uint32 {
	k := uint(n & 31)
	if k == 0 {
		return value
	}
	return bits.RotateLeft32(value, int(k))
}

func hongguoRor32(value uint32, n int) uint32 {
	k := n & 31
	if k == 0 {
		return value
	}
	return bits.RotateLeft32(value, -k)
}

func hongguoGetIV(iv uint32, data []byte) uint32 {
	value := iv
	for index, current := range data {
		if index&1 == 0 {
			value = (value >> 4) ^ value ^ (value << 6) ^ uint32(current)
		} else {
			value = ^((value >> 7) ^ value ^ (uint32(current) | (value << 12)))
		}
	}
	return value
}

func hongguoSumMD5(data []byte) uint32 {
	check := uint32(0x20220420)
	for index := 0; index < 12; index++ {
		temp := (check >> 5) ^ check
		if index&1 == 0 {
			temp = (check >> 3) ^ check
			check = uint32(data[index]) ^ (check << 7)
		} else {
			check = uint32(data[index]) | (check << 11)
			check = ^check
		}
		check ^= temp
	}
	return (check | 4) ^ 0x1000000
}

func hongguoRandInt(limit int) int {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(limit)))
	if err != nil {
		return 0
	}
	return int(value.Int64())
}

func hongguoRandInt32() int32 {
	var raw [4]byte
	_, _ = rand.Read(raw[:])
	return int32(binary.BigEndian.Uint32(raw[:]))
}

func hongguoRandBytes(count int) []byte {
	out := make([]byte, count)
	_, _ = rand.Read(out)
	return out
}

func hongguoSignParam(query, key, fallback string) string {
	for _, pair := range stringsSplit(query, "&") {
		eq := indexByte(pair, '=')
		if eq <= 0 || pair[:eq] != key {
			continue
		}
		if value := pair[eq+1:]; value != "" {
			return value
		}
	}
	return fallback
}

func stringsSplit(value, sep string) []string {
	if value == "" {
		return nil
	}
	return splitKeep(value, sep)
}

func splitKeep(value, sep string) []string {
	var out []string
	for {
		index := indexString(value, sep)
		if index < 0 {
			return append(out, value)
		}
		out = append(out, value[:index])
		value = value[index+len(sep):]
	}
}

func indexByte(value string, char byte) int {
	for index := 0; index < len(value); index++ {
		if value[index] == char {
			return index
		}
	}
	return -1
}

func indexString(value, part string) int {
	if len(part) == 0 || len(part) > len(value) {
		return -1
	}
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return index
		}
	}
	return -1
}

func hongguoBlockPad(value []byte) []byte {
	pad := 16 - (len(value) % 16)
	if len(value)%16 == 0 {
		pad = 16
	}
	out := make([]byte, len(value)+pad)
	copy(out, value)
	for index := len(value); index < len(out); index++ {
		out[index] = byte(pad)
	}
	return out
}
