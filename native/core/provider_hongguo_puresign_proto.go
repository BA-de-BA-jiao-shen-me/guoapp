package core

import (
	"encoding/binary"
	"math"
)

func hongguoProtoJoin(parts ...[]byte) []byte {
	size := 0
	for _, part := range parts {
		size += len(part)
	}
	out := make([]byte, 0, size)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

func hongguoProtoVarint(value uint64) []byte {
	var out []byte
	for value > 127 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

func hongguoProtoZigzag(value int64) uint64 {
	if value < 0 {
		return uint64(-value)*2 - 1
	}
	return uint64(value) * 2
}

func hongguoProtoKey(tag int, wire uint64) []byte {
	return hongguoProtoVarint(uint64(tag)<<3 | wire)
}

func hongguoProtoSint(tag int, value int64) []byte {
	return append(hongguoProtoKey(tag, 0), hongguoProtoVarint(hongguoProtoZigzag(value))...)
}

func hongguoProtoString(tag int, value string) []byte {
	if value == "" {
		return nil
	}
	body := []byte(value)
	return append(append(hongguoProtoKey(tag, 2), hongguoProtoVarint(uint64(len(body)))...), body...)
}

func hongguoProtoBytes(tag int, value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return append(append(hongguoProtoKey(tag, 2), hongguoProtoVarint(uint64(len(value)))...), value...)
}

func hongguoProtoMessage(tag int, value []byte) []byte {
	return hongguoProtoBytes(tag, value)
}

func hongguoProtoFloat(tag int, value float32) []byte {
	body := make([]byte, 4)
	binary.LittleEndian.PutUint32(body, math.Float32bits(value))
	return append(hongguoProtoKey(tag, 5), body...)
}

func hongguoDeviceProto(deviceID string) []byte {
	return hongguoProtoJoin(
		hongguoProtoSint(1, 1), hongguoProtoSint(2, 2), hongguoProtoString(3, "8662"), hongguoProtoString(4, deviceID),
		hongguoProtoString(5, hongguoProcessNonce), hongguoProtoString(6, "!noperm!"),
		hongguoProtoSint(7, -888888), hongguoProtoSint(8, -888888), hongguoProtoSint(9, 3), hongguoProtoSint(10, -888888),
		hongguoProtoString(11, "!notset!"), hongguoProtoString(12, "Asia/Shanghai,8"), hongguoProtoString(13, "zh_CN"),
		hongguoProtoSint(14, 4),
		hongguoProtoFloat(16, 0), hongguoProtoFloat(17, 0), hongguoProtoFloat(18, 0), hongguoProtoFloat(19, 0),
		hongguoProtoFloat(20, 0), hongguoProtoFloat(21, 0),
		hongguoProtoString(22, "16"), hongguoProtoSint(23, 41), hongguoProtoSint(24, 36),
		hongguoProtoSint(25, hongguoProcessStarted), hongguoProtoSint(26, hongguoProcessStarted),
		hongguoProtoSint(27, hongguoProcessStarted), hongguoProtoSint(28, hongguoProcessStarted),
		hongguoProtoSint(29, -1),
		hongguoProtoString(30, "25053RT47C"), hongguoProtoString(31, "Redmi"), hongguoProtoString(32, "25053RT47C"),
		hongguoProtoString(33, "25053RT47C"), hongguoProtoString(34, "Xiaomi"), hongguoProtoString(35, "Redmi"),
		hongguoProtoString(36, "Redmi"), hongguoProtoSint(38, 31),
	)
}
