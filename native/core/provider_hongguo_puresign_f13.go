package core

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
)

var hongguoBranch2SV = []uint32{
	0xa7aefe20, 0x7149f1d6, 0x47e4ca07, 0xe9b58f67, 0x93b924de, 0xc614d0f5, 0x38afe0ef, 0xb2bbad73,
	0xe24444c3, 0x9d3aec9b, 0xdf7b37e4, 0xd8b16d40, 0xf8ac31b8, 0x76b9a90b, 0x31d833ee, 0x953fce64,
	0x353595a4, 0x4609c13b, 0x36925008, 0x8c6d0925, 0x5df5c177, 0x1cfbf52b, 0x8a4fa7f0, 0x114ca35e,
	0x8193f984, 0x7a7a8733, 0x316ab4d5, 0x3c20cfc9, 0xa6d84453, 0x3a18500c, 0x798ec47a, 0x97a76b28,
	0x66c4ff96, 0x51716443, 0x0dd2fc3b, 0xb5696da7, 0xbbeb3ac5, 0x5c53d204, 0xd32608ce, 0x7279b9ec,
	0xf4188ecf, 0xf7d793db, 0x332cc491, 0xab76ae15, 0x9bebe727, 0x18a01384, 0x5be9f8a7, 0x5f90a754,
	0x39b663c0, 0x36673c83, 0x7c92f514, 0x9d7d94d7, 0xe2e8d9aa, 0x5f7e9ea9, 0x7abd4551, 0x569e05da,
	0x40a25632, 0x3df5a9a5, 0xbab37d80, 0x454286dc, 0x3f5d4e78, 0x03d7b75d, 0xb1fe4af7, 0xa5ab26a3,
}

var hongguoBranch2Orders = func() []byte {
	raw, _ := hex.DecodeString(
		"0f0704000908030a060b050d0e010c02" +
			"0f05080c0009020103070e060b0a0d04" +
			"060500070c000a04080f010b0d09020e" +
			"060b02050403080101070a000d0c090e" +
			"0d070e0f0b0208030c0509010004060a" +
			"0d0902060f0b0a040807000c0503010e" +
			"0c090f07060f030e020d0405010b0a00" +
			"0c050a090e0802040407030f01060b00" +
			"0b0f0408020a0700090d06010e03050c" +
			"0b060a0508020c03070f0e090d000104" +
			"0906080f050800040a0b030d01020c0e" +
			"090d0c0604070a03030f00080105020e" +
			"01000d0f090a0b0e0402080703060c05" +
			"01080a0c0f0905060b000304020e070d" +
			"04080f000c0f0e0d0a01060207090503" +
			"040205080d0b0a0606000e0f070c0903" +
			"0a08040f000b01060d0c0709030e0502" +
			"0a070b050f00020e0108030d0c060904" +
			"0d070f08050f06040b0a0e0c09000203" +
			"0d0c020704010b0e0e08060f09050003")
	return raw
}()

func hongguoHashF13(querySM3, bodyMD5, tsBytes []byte, khronos uint32) []byte {
	iv := hongguoGetIV(hongguoGetIV(hongguoGetIV(0x20230928, querySM3), bodyMD5), tsBytes)
	low := iv & 15
	ivV0 := (low * 171) >> 9
	branch := low - ivV0*3
	if branch == 2 {
		return hongguoBranch2F13(iv, querySM3, bodyMD5, tsBytes, khronos)
	}
	if branch != 0 {
		head := append(append([]byte{}, querySM3[:16]...), bodyMD5...)
		return append(head, hongguoLe32(hongguoSumMD5(append(append([]byte{}, querySM3...), bodyMD5...)))...)
	}
	ivV1List := []uint32{0xc4a78580, 0xb3c0fd39, 0xc58c5686, 0xc9aa3ba7, 0xf5a7adf2, 0x963c2ed1}
	ivV1 := ivV1List[ivV0]
	countV1 := (ivV1 + khronos + 1) & 255
	countV2 := ivV1 + khronos
	tt02 := []uint32{
		0xebb64faf, 0x07aadcc2, 0xcf3187bf, 0xe01138ff, 0x6d0bfcff, 0x5a30a3be, 0xb41ad638, 0x34180eb8,
		0xf233eb6f, 0xb1a584cc, 0xccc30dc7, 0x47d1db51, 0xd55653de, 0x70a84fa1, 0x57473c12, 0xf76f0288,
		0x2c077f0a, 0xda0dcad0, 0xfbb86f6c, 0xfdc4cf00, 0x688a020d, 0xe676c6a6, 0x8cd6338b, 0x1a3c8d0e,
		0xcce8b06b, 0x6ad0ed0b, 0xa0522717, 0xdc71ac83, 0x2285db71, 0xd5b4dda6, 0x736f8650, 0x6560306c,
		0x617ce2a6, 0xe423417e, 0x0a40e143, 0x544e4032, 0x88dffb2a, 0x716c1ae0, 0x4c467a88, 0x05b23bb3,
		0xe1d0b866, 0xbaa3dcb8, 0xae3374d3, 0xc3381a50, 0x1702f75b, 0xfe6da368, 0xf0b4cf48, 0x4e0ffbb8,
		0x72aad10d, 0x26c53a3d, 0xf2bce0f6, 0xb4557581, 0x4a257fdd, 0x8c3182a2, 0xab0b3b86, 0x3d5dfb14,
		0x4f103634, 0xd37b52d7, 0x444eff16, 0xeb0a33d1, 0x6ca86f6e, 0x0284ba7, 0x08387cfa, 0x5fb37586,
	}
	tt := make([]uint32, len(tt02))
	for index, value := range tt02 {
		tt[index] = hongguoRor32(value, int(countV1))
	}
	n0 := (countV2 + 2) & 7
	seed := []byte{0xfa, 0x45, 0x61, 0xd7}
	pad := make([]byte, len(seed))
	for index, value := range seed {
		pad[index] = byte(((uint32(value) | (uint32(value) << 8)) >> n0) & 255)
	}
	data := append(append(append(append(append([]byte{}, querySM3...), bodyMD5...), tsBytes...), pad...), 0, 0, 0, 0, 0, 0, 0x01, 0xa0)
	di := make([]uint32, len(data)/4+112+16)
	diLen := 0
	for index := 0; index+3 < len(data); index += 4 {
		di[diLen] = binary.BigEndian.Uint32(data[index:])
		diLen++
	}
	di0 := di[0]
	for index := 0; index < 112; index++ {
		di1 := di[index+1]
		di14 := di[index+14]
		r1 := hongguoRotl32(di1, 14) ^ hongguoRotl32(di1, 25) ^ (di1 >> 3)
		r2 := hongguoRotl32(di14, 13) ^ hongguoRotl32(di14, 15) ^ (di14 >> 10)
		di[diLen] = di0 + di[index+9] + r1 + r2
		diLen++
	}
	initRaw := []uint32{0x7aba4fc8, 0x67166507, 0x6403fa00, 0x340f512f, 984304912, 3005047866, 2874125293, 2152413264}
	init := make([]uint32, 8)
	for index, value := range initRaw {
		init[index] = hongguoRor32(value, int(countV2&31))
	}
	chosenTable := [][]int{
		{101, 5, 7, 6, 3, 2, 1, 0, 5, 4, 3},
		{96, 0, 6, 7, 5, 3, 2, 1, 5, 4, 4},
		{96, 7, 6, 2, 1, 4, 0, 5, 4, 3, 5},
		{99, 3, 6, 2, 4, 5, 1, 0, 0, 7, 6},
		{96, 0, 5, 6, 7, 3, 1, 2, 5, 4, 4},
		{100, 2, 0, 3, 5, 4, 6, 7, 2, 1, 5},
	}
	chosen := chosenTable[ivV0]
	d := append([]uint32{}, init...)
	for index := 0; index < chosen[0]; index++ {
		base := int(ivV1) + index
		n0v := di[base&127]
		n1 := ((d[chosen[3]] ^ d[chosen[4]]) & d[chosen[1]]) ^ d[chosen[3]]
		n2 := hongguoRotl32(d[chosen[1]], 26) ^ hongguoRotl32(d[chosen[1]], 21) ^ hongguoRotl32(d[chosen[1]], 7)
		n4 := n0v + n1 + n2 + tt[base&63] + d[chosen[5]]
		n5 := hongguoRotl32(d[chosen[2]], 30) ^ hongguoRotl32(d[chosen[2]], 19) ^ hongguoRotl32(d[chosen[2]], 10)
		n6 := (d[chosen[2]] & d[chosen[6]]) | ((d[chosen[2]] | d[chosen[6]]) & d[chosen[7]])
		n7 := n5 + n6
		old := d[chosen[9]]
		last := d[len(d)-1]
		copy(d[1:], d[:len(d)-1])
		d[0] = last
		d[chosen[10]] = n7 + n4
		d[chosen[8]] = old + n4
	}
	ret := make([]byte, 32)
	for index := 0; index < 8; index++ {
		binary.BigEndian.PutUint32(ret[index*4:], d[index]+init[index])
	}
	for index := 0; index < 16; index++ {
		ret[index] ^= ret[index+16]
	}
	return append(append([]byte{}, ret[:16]...), hongguoLe32(hongguoSumMD5(ret[:16]))...)
}

func hongguoBranch2F13(iv uint32, querySM3, bodyMD5, tsBytes []byte, khronos uint32) []byte {
	v := (iv & 13) * 86
	ivV0 := ((v >> 15) & 255) + ((v >> 8) & 255)
	n1 := []uint32{0x8980f29b, 0xeb549c7f, 0xb08726db, 0xd40cb5e6, 0xe8f559e4}[ivV0]
	countV1 := (n1 + khronos + 1) & 255
	countV2 := n1 + khronos
	shift := (countV2 + 5) & 7
	seed := []byte{0x84, 0x96, 0x77, 0x9d, 0xd4, 0x15, 0x0b, 0xf8}
	pad := make([]byte, 8)
	for index, value := range seed {
		pad[index] = byte(((uint32(value) | (uint32(value) << 8)) >> shift) & 255)
	}
	input := append(append(append(append(append([]byte{}, querySM3...), bodyMD5...), tsBytes...), pad...), 0xa0, 0x01, 0, 0)
	from := int(ivV0) << 6
	orders := hongguoBranch2Orders[from : from+64]
	return hongguoMD5SumV3(input, countV2, orders, countV1)
}

func hongguoMD5SumV3(message []byte, countV2 uint32, orders []byte, countV1 uint32) []byte {
	sv := make([]uint32, len(hongguoBranch2SV))
	for index, value := range hongguoBranch2SV {
		sv[index] = hongguoRor32(value, int(countV1))
	}
	start := []uint32{
		hongguoRor32(0x79e0f2fb, int(countV2)), hongguoRor32(0xc8b52570, int(countV2)),
		hongguoRor32(0xebc2f8cd, int(countV2)), hongguoRor32(0x7c104d93, int(countV2)),
	}
	endCount := (countV2 + 6) & 255
	end := []uint32{
		hongguoRor32(0x19be4866, int(endCount)), hongguoRor32(0xe85986b4, int(endCount)),
		hongguoRor32(0xe19b326e, int(endCount)), hongguoRor32(0x71d1d7d4, int(endCount)),
	}
	var m [16]uint32
	for index := 0; index < 16; index++ {
		m[index] = binary.LittleEndian.Uint32(message[index*4:])
	}
	a, b, c, d := start[0], start[1], start[2], start[3]
	shifts := [4][16]int{
		{7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22},
		{5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20},
		{4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23},
		{6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21},
	}
	for kind := 0; kind < 4; kind++ {
		for index := 0; index < 16; index++ {
			phase := index & 3
			mm := m[orders[kind*16+index]]
			cc := sv[kind*16+index]
			var next uint32
			switch phase {
			case 0:
				next = hongguoMD5V3Step(kind, a, b, c, d, mm, shifts[kind][index], cc)
				a = next
			case 1:
				next = hongguoMD5V3Step(kind, d, a, b, c, mm, shifts[kind][index], cc)
				d = next
			case 2:
				next = hongguoMD5V3Step(kind, c, d, a, b, mm, shifts[kind][index], cc)
				c = next
			default:
				next = hongguoMD5V3Step(kind, b, c, d, a, mm, shifts[kind][index], cc)
				b = next
			}
		}
	}
	ret := make([]byte, 16)
	binary.LittleEndian.PutUint32(ret[0:], (start[0]+a)^end[0])
	binary.LittleEndian.PutUint32(ret[4:], (start[1]+b)^end[1])
	binary.LittleEndian.PutUint32(ret[8:], (start[2]+c)^end[2])
	binary.LittleEndian.PutUint32(ret[12:], (start[3]+d)^end[3])
	return append(ret, hongguoLe32(hongguoSumMD5(ret))...)
}

func hongguoMD5V3Step(kind int, a, b, c, d, m uint32, shift int, constant uint32) uint32 {
	var f uint32
	switch kind {
	case 0:
		f = (b & c) | (^b & d)
	case 1:
		f = (b & d) | (c & ^d)
	case 2:
		f = b ^ c ^ d
	default:
		f = c ^ (b | ^d)
	}
	return b + hongguoRotl32(a+f+m+constant, shift)
}

var hongguoAesSBox = func() []byte {
	raw, _ := base64.StdEncoding.DecodeString(
		"+n0Ia5xZs0sEXznQOEqRmQBnpiCf9U2Ccybu3xhmgzOAAxn72f6uqqmwUsYL83klTni0NqxdGieeiNu9PGPsSRXBMB/cuFbUbM3KCUPINaPvHvSW0vwOcnuUhNHqRVpiAj/TEoE0K91+5ijypUYTATsh9mE3KSoN7Yyvv51cuyR2D3XkU4nhmI2xmmVwT1RMWKtub4sjxAcRDLrPoKSO2AU9FLLadMPX577Wf95IFj6FkKFVt3dCIsmGUC4X+WQxLJvxbRxEaOPpqJOXyzJX6+Vxaq3AzMfF/WAdoi1Hp+JRaV56zgpBtpWP97mH4DoGEIq1+FvV8LyS/3wvwugbQOwb2r26mJEMsiuDQTRn+wrYdrVGBVlhI3WQhyrjUBVMrLF5667llUcEaPCGPVGLD8qO5LlO8hKCvA7V9+8oJc9bXelqVQLhM76T5/WtnT45JKji+hdX0HoNCDDWuKON/QeaxB5uImSX0h2wv0VmP2zd2yeApxHcpsVS+MC2yFwAc2B7oBkTqsk1SEvTpM2fmfMQREBUfin0Bh+iq6EvPPavhWI2IX9e3yAas7Tm/3KEj2UmlFp36kN4x0rMLBRrxuh0U/zUHM4xcAMYjJY4MonxOl/X+alpt2M3WMI7w3HLnpIBigtNiJu7T21v4P6lSd5WFgntnMEt7oF92XzRLkJbTcGmXepE/UVOG6E/0YnhfS+q26utWcuxzpooyeD2cDlK1/8w9d28VzsRjbLuALbmGlp8+d7EzS6Au7lMpZ+ECMZvQmzwJ+eLOpxR+2chdUExp8ogQyq3v9l68rX4jCwjg0+PYKAEEzcU4wHFY2ZcdIHfWL1okD3SszT0GZMyKdZJrg1L2AeerB4tC0C4crp2EHGo5FYdSP7lwkeR2ocmnR+Ia8CYviUJlzOjhRZef9xuVOn3qcjow3fQgivsAmKKkg4+sA8F8/GWeDiGNhg8JM8KtFPMYWWkx5TVFX5t73kiNRJqjlIGVXtGZFCV4gzt0xcDopuZ6xz8r9Rzafpf9ywev8jh8592gHFIqpStZPuJxmDDMrNN0uBE3V+oscdoIzTJbRJ/t+sVvqnReJOgDJKk10fjisJwqyZBmnmn2BSFj8BvVtCMEbkuPOKdzw7eA11GPs04Qw8zWtkaZWwiO/wwpojqN6K0jY5RnNZA7vn4hPSul+nKCkVnVwQvg1zVxcSCtqORmB9KrJaBbssbCQivGJVJfVTt+hYxOtq4ZvWl8f4QAQZ0zGPffCgl9s6yT4vlvIdpu4YhBwA25wtQWZsc6GJYGWHyvSdeuh3mmUI9DSq13Fsp8C1MU3tqc04/df9LoTUXVXI5INOw/e8C7Hd+5CvbkMEFnnrUUmskEw==")
	return raw
}()

type hongguoAesV3 struct {
	wordSize int
	box      []byte
	con2     []int
	order    []int
	keys     [][]byte
}

func newHongguoAesV3(key []byte, khronos uint32) *hongguoAesV3 {
	wordSize := int(khronos & 3)
	aes := &hongguoAesV3{
		wordSize: wordSize,
		box:      hongguoAesSBox[wordSize*256 : (wordSize+1)*256],
		con2:     [][]int{{1, 0, 2, 3}, {2, 0, 3, 1}, {0, 1, 3, 2}, {1, 0, 2, 3}}[wordSize],
		order: [][]int{
			{0, 9, 14, 11, 4, 13, 2, 7, 8, 1, 6, 15, 12, 5, 10, 3},
			{0, 9, 14, 15, 4, 13, 2, 7, 8, 1, 6, 3, 12, 5, 10, 11},
			{0, 9, 14, 7, 4, 13, 2, 11, 8, 1, 6, 3, 12, 5, 10, 15},
			{0, 9, 14, 11, 4, 13, 2, 7, 8, 1, 6, 15, 12, 5, 10, 3},
		}[wordSize],
	}
	init := []uint32{0xca025ddc, 0x823dc546, 0xc9420583, 0xc298225f}[wordSize]
	initial := make([]byte, 16)
	for index := 0; index < 4; index++ {
		binary.LittleEndian.PutUint32(initial[index*4:], init)
	}
	expanded := make([]byte, 48)
	for index := 0; index < 16; index++ {
		expanded[index] = initial[index] ^ key[index]
	}
	rounds := 8
	for index := 4; index < 12; index++ {
		at := 4 * (index - 1)
		a, b, c, d := int(expanded[at]), int(expanded[at+1]), int(expanded[at+2]), int(expanded[at+3])
		if index&3 == 0 {
			t := (int(init>>uint(rounds&24)) ^ int(aes.box[b])) & 255
			b, c, d = int(aes.box[c]), int(aes.box[d]), int(aes.box[a])
			a = t
		}
		rounds += 2
		expanded[at+4] = byte(a ^ int(expanded[at-12]))
		expanded[at+5] = byte(b ^ int(expanded[at-11]))
		expanded[at+6] = byte(c ^ int(expanded[at-10]))
		expanded[at+7] = byte(d ^ int(expanded[at-9]))
	}
	aes.keys = make([][]byte, 12)
	for index := 0; index < 12; index++ {
		aes.keys[index] = append([]byte{}, expanded[index*4:index*4+4]...)
	}
	return aes
}

func (aes *hongguoAesV3) subKeys(from int) [][]byte {
	out := make([][]byte, 4)
	for index := 0; index < 4; index++ {
		out[index] = append([]byte{}, aes.keys[from+index]...)
	}
	return out
}

func (aes *hongguoAesV3) encrypt(data, iv []byte) []byte {
	plaintext := make([]byte, 32)
	for index := 0; index < 31; index++ {
		at := index * 8
		n0 := (hongguoAesSrc(data, at) >> 4) & 2
		n1 := n0 | (hongguoAesSrc(data, at+1) & 64)
		n2 := n1 | ((hongguoAesSrc(data, at+2) >> 2) & 1)
		n3 := n2 | ((hongguoAesSrc(data, at+3) << 3) & 128)
		n4 := n3 | ((hongguoAesSrc(data, at+4) >> 1) & 4)
		n5 := n4 | ((hongguoAesSrc(data, at+5) << 3) & 16)
		n6 := n5 | ((hongguoAesSrc(data, at+6) << 5) & 32)
		plaintext[index] = byte(n6 | ((hongguoAesSrc(data, at+7) >> 4) & 8))
	}
	plaintext[31] = 1
	var blocks []byte
	previous := append([]byte{}, iv...)
	for index := 0; index < len(plaintext); index += 16 {
		value := aes.block(hongguoXor(plaintext[index:index+16], previous))
		blocks = append(blocks, value...)
		previous = value
	}
	out := append([]byte{}, data...)
	for index := 0; index < 31; index++ {
		at := index * 8
		k := int(blocks[index])
		hongguoAesPut(out, at, (hongguoAesSrc(out, at)&0xdf)|((k<<4)&32))
		hongguoAesPut(out, at+1, (hongguoAesSrc(out, at+1)&0xbf)|(k&64))
		hongguoAesPut(out, at+2, (hongguoAesSrc(out, at+2)&0xfb)|((k<<2)&4))
		hongguoAesPut(out, at+3, (hongguoAesSrc(out, at+3)&0xef)|((k>>3)&16))
		hongguoAesPut(out, at+4, (hongguoAesSrc(out, at+4)&0xf7)|((k+k)&8))
		hongguoAesPut(out, at+5, (hongguoAesSrc(out, at+5)&0xfd)|((k>>3)&2))
		hongguoAesPut(out, at+6, (hongguoAesSrc(out, at+6)&0xfe)|((k>>5)&1))
		hongguoAesPut(out, at+7, (hongguoAesSrc(out, at+7)&0x7f)|((k<<4)&128))
	}
	return append([]byte{blocks[len(blocks)-1]}, out...)
}

func (aes *hongguoAesV3) block(value []byte) []byte {
	rows := hongguoAesMatrix(value)
	aes.addCon(rows, aes.subKeys(0))
	for index := 1; index < 3; index++ {
		aes.sub(rows)
		aes.shift(rows)
		if index == 1 {
			aes.shiftCon(rows)
			hongguoAesMix(rows)
		}
		aes.addCon(rows, aes.subKeys(index*4))
	}
	aes.add(rows, aes.subKeys(4))
	return hongguoAesFlat(rows)
}

func (aes *hongguoAesV3) add(rows [][]byte, key [][]byte) {
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i][j] ^= key[i][j]
		}
	}
}

func (aes *hongguoAesV3) addCon(rows [][]byte, key [][]byte) {
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i][j] ^= key[i][aes.con2[j]]
		}
	}
}

func (aes *hongguoAesV3) sub(rows [][]byte) {
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i][j] = aes.box[rows[i][j]]
		}
	}
	old := make([][]byte, 4)
	for i := 0; i < 4; i++ {
		old[i] = append([]byte{}, rows[i]...)
	}
	for i := 0; i < 4; i++ {
		rows[i] = old[aes.con2[i]]
	}
}

func (aes *hongguoAesV3) shift(rows [][]byte) {
	flat := hongguoAesFlat(rows)
	for index := 0; index < 16; index++ {
		rows[index/4][index%4] = flat[aes.order[index]]
	}
}

func (aes *hongguoAesV3) shiftCon(rows [][]byte) {
	old := make([][]byte, 4)
	for i := 0; i < 4; i++ {
		old[i] = append([]byte{}, rows[i]...)
	}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i][j] = old[i][aes.con2[j]]
		}
	}
}

func hongguoAesMix(rows [][]byte) {
	for i := 0; i < 4; i++ {
		t := rows[0][i] ^ rows[1][i] ^ rows[2][i] ^ rows[3][i]
		u := rows[0][i]
		rows[0][i] ^= t ^ hongguoAesXt(rows[0][i]^rows[1][i])
		rows[1][i] ^= t ^ hongguoAesXt(rows[1][i]^rows[2][i])
		rows[2][i] ^= t ^ hongguoAesXt(rows[2][i]^rows[3][i])
		rows[3][i] ^= t ^ hongguoAesXt(rows[3][i]^u)
	}
}

func hongguoAesXt(value byte) byte {
	extra := byte(0)
	if value&128 != 0 {
		extra = 0x1b
	}
	return (value << 1) ^ extra
}

func hongguoAesMatrix(value []byte) [][]byte {
	rows := make([][]byte, 4)
	for i := 0; i < 4; i++ {
		rows[i] = append([]byte{}, value[i*4:i*4+4]...)
	}
	return rows
}

func hongguoAesFlat(rows [][]byte) []byte {
	out := make([]byte, 16)
	for i := 0; i < 4; i++ {
		copy(out[i*4:], rows[i])
	}
	return out
}

func hongguoAesSrc(data []byte, index int) int {
	if index < 0 || index >= len(data) {
		return 0
	}
	return int(data[index])
}

func hongguoAesPut(out []byte, index, value int) {
	if index >= 0 && index < len(out) {
		out[index] = byte(value)
	}
}

func hongguoXor(left, right []byte) []byte {
	out := append([]byte{}, left...)
	for index := range out {
		out[index] ^= right[index%len(right)]
	}
	return out
}
