package ethereum

import "encoding/binary"

const keccakRate = 136

var keccakRoundConstants = [24]uint64{
	0x0000000000000001, 0x0000000000008082,
	0x800000000000808a, 0x8000000080008000,
	0x000000000000808b, 0x0000000080000001,
	0x8000000080008081, 0x8000000000008009,
	0x000000000000008a, 0x0000000000000088,
	0x0000000080008009, 0x000000008000000a,
	0x000000008000808b, 0x800000000000008b,
	0x8000000000008089, 0x8000000000008003,
	0x8000000000008002, 0x8000000000000080,
	0x000000000000800a, 0x800000008000000a,
	0x8000000080008081, 0x8000000000008080,
	0x0000000080000001, 0x8000000080008008,
}

var keccakRotation = [25]uint{
	0, 1, 62, 28, 27,
	36, 44, 6, 55, 20,
	3, 10, 43, 25, 39,
	41, 45, 15, 21, 8,
	18, 2, 61, 56, 14,
}

func keccak256(data []byte) [32]byte {
	var state [25]uint64

	for len(data) >= keccakRate {
		absorbKeccakBlock(&state, data[:keccakRate])
		keccakF1600(&state)
		data = data[keccakRate:]
	}

	var block [keccakRate]byte
	copy(block[:], data)
	block[len(data)] = 0x01 // legacy Keccak domain separator
	block[keccakRate-1] |= 0x80
	absorbKeccakBlock(&state, block[:])
	keccakF1600(&state)

	var out [32]byte
	for i := 0; i < len(out)/8; i++ {
		binary.LittleEndian.PutUint64(out[i*8:], state[i])
	}
	return out
}

func absorbKeccakBlock(state *[25]uint64, block []byte) {
	for i := 0; i < keccakRate/8; i++ {
		state[i] ^= binary.LittleEndian.Uint64(block[i*8:])
	}
}

func keccakF1600(a *[25]uint64) {
	for _, rc := range keccakRoundConstants {
		var c [5]uint64
		var d [5]uint64
		var b [25]uint64

		for x := 0; x < 5; x++ {
			c[x] = a[x] ^ a[x+5] ^ a[x+10] ^ a[x+15] ^ a[x+20]
		}
		for x := 0; x < 5; x++ {
			d[x] = c[(x+4)%5] ^ rotateLeft64(c[(x+1)%5], 1)
		}
		for y := 0; y < 5; y++ {
			for x := 0; x < 5; x++ {
				a[x+5*y] ^= d[x]
			}
		}

		for y := 0; y < 5; y++ {
			for x := 0; x < 5; x++ {
				source := x + 5*y
				targetX := y
				targetY := (2*x + 3*y) % 5
				b[targetX+5*targetY] = rotateLeft64(a[source], keccakRotation[source])
			}
		}

		for y := 0; y < 5; y++ {
			for x := 0; x < 5; x++ {
				a[x+5*y] = b[x+5*y] ^ (^b[(x+1)%5+5*y] & b[(x+2)%5+5*y])
			}
		}

		a[0] ^= rc
	}
}

func rotateLeft64(value uint64, shift uint) uint64 {
	if shift == 0 {
		return value
	}
	return value<<shift | value>>(64-shift)
}
