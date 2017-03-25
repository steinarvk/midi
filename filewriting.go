package midi

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func encodeVarint(n uint64) []byte {
	if n == 0 {
		return []byte{0}
	}

	var rrv []byte
	for n > 0 {
		b := byte(n & 0x7f)
		n = n >> 7
		rrv = append(rrv, b)
	}

	var rv []byte

	for i := len(rrv) - 1; i >= 0; i-- {
		b := rrv[i]
		if i != 0 {
			b |= 0x80
		}
		rv = append(rv, b)
	}

	return rv
}

func (t *Track) encode() ([]byte, error) {
	data, err := encodeEvents(t.Events)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	buf.WriteString("MTrk")
	var chunkLen uint32 = uint32(len(data))
	if err := binary.Write(buf, binary.BigEndian, chunkLen); err != nil {
		return nil, err
	}
	buf.Write(data)
	return buf.Bytes(), nil
}

func (h *Header) encode() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteString("MThd")
	var headerLen uint32 = 6
	if err := binary.Write(buf, binary.BigEndian, headerLen); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, h); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (f *File) encode() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	data, err := f.Header.encode()
	if err != nil {
		return nil, err
	}
	buf.Write(data)

	for i, trk := range f.Tracks {
		data, err = trk.encode()
		if err != nil {
			return nil, fmt.Errorf("error encoding track #%d: %v", i, err)
		}
		buf.Write(data)
	}

	return buf.Bytes(), nil
}

func encodeEvents(evts []Event) ([]byte, error) {
	var rv []byte

	var delay uint64

	for i, evt := range evts {
		td, ok := evt.(TimeDeltaEvent)
		if ok {
			delay += uint64(td)
			continue
		}

		rv = append(rv, encodeVarint(delay)...)
		delay = 0

		encoded, err := evt.EncodeMIDI()
		if err != nil {
			return nil, fmt.Errorf("error encoding event #%d (%v): %v", i, evt, err)
		}

		rv = append(rv, encoded...)
	}

	return rv, nil
}
