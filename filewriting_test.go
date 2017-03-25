package midi

import (
	"bytes"
	"testing"
)

func TestVarintEncoding(t *testing.T) {
	numbers := []uint64{
		12345678,
		12328,
		34278,
		123793,
		0,
		342,
	}
	for _, n := range numbers {
		encoded := encodeVarint(n)
		val, err := readVarint(bytes.NewBuffer(encoded))
		if err != nil {
			t.Errorf("readVarint(..encodeVarint(%d)=%v..) = err: %v", n, encoded, err)
			continue
		}
		if val != n {
			t.Errorf("readVarint(..encodeVarint(%d)=%v..) = %v want %v", n, encoded, val, n)
		}
	}
}
