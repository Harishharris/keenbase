package keenbase

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
)

const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
const idLength = 15

func newID() string {
	alphabetLen := big.NewInt(int64(len(idAlphabet)))
	buf := make([]byte, idLength)

	for i := range buf {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			// Fallback: read raw bytes and reduce modulo (still random, just
			// slightly biased — acceptable for IDs, never for crypto keys).
			var b [1]byte
			_, _ = rand.Read(b[:])
			buf[i] = idAlphabet[int(binary.BigEndian.Uint16([]byte{0, b[0]}))%len(idAlphabet)]
			continue
		}
		buf[i] = idAlphabet[n.Int64()]
	}

	return string(buf)
}
