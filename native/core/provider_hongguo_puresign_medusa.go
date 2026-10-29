package core

func hongguoSM3Bytes(data []byte) []byte {
	sum := hongguoSM3(data)
	return sum[:]
}

func hongguoKeyHash(signKey []byte, random int32) ([]byte, []byte) {
	hash := hongguoSM3Bytes(append(append(append([]byte{}, signKey...), hongguoLe32(uint32(random))...), signKey...))
	d1 := byte(uint32(random) >> 16)
	d2 := ((uint32(d1) << 11) | (uint32(random) >> 24)) ^ (uint32(d1) >> 5) ^ uint32(d1)
	return hash, hongguoLe32(^d2)
}

func hongguoXMXOR(data, key []byte) []byte {
	encoded := make([]byte, len(data))
	for index := range data {
		at := (index * 4) & 28
		d0 := uint32(key[at])
		d1 := uint32(key[at+1])
		d2 := uint32(hongguoRotl8(data[index])) + d0
		d2 = (^d2) ^ d1
		d2 = uint32(hongguoRotl8Three(byte(d2)))
		d2 = (d2 + d1) & 255
		d2 = (d2 ^ d0) & 255
		encoded[len(data)-index-1] = ^byte(d2)
	}
	last := encoded[len(encoded)-1] ^ encoded[len(encoded)-2]
	first := encoded[0]
	encoded[0] = (^last) + first
	encoded[1] = ((encoded[0] ^ encoded[len(encoded)-1] ^ 254) + encoded[1])
	encoded[2] = encoded[2] + (((last - first) ^ hongguoRotl8Three(encoded[1])) ^ 2)
	for index := 0; index < len(encoded)-4; index++ {
		temp := hongguoRotl8Three(encoded[index+2]) ^ encoded[index+1] ^ byte(index+3)
		encoded[index+3] = (^temp) + encoded[index+3]
	}
	encoded[len(encoded)-1] ^= encoded[len(encoded)-2]
	sum := 0
	for index := 0; index < len(encoded)-1; index++ {
		sum += int(encoded[index+1])
	}
	encoded[0] = (encoded[0] ^ encoded[1]) + byte(sum)
	return encoded
}
