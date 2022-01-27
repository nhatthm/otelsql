package suite

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"math/rand"
)

func randomString(length int) string {
	var rngSeed int64

	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	r := rand.New(rand.NewSource(rngSeed))

	result := make([]byte, length/2)

	r.Read(result[:])

	return hex.EncodeToString(result[:])
}
