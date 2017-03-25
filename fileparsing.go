package midi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/steinarvk/midi/contextreader"
	"github.com/steinarvk/midi/limitreader"
)

type Header struct {
	Format         uint16
	NumberOfTracks uint16
	Division       int16
}

type Track struct {
	Events []Event
}

type File struct {
	Header *Header
	Tracks []*Track
}

func readLiteralExpecting(r io.Reader, s string) error {
	buf := make([]byte, len(s))
	n, err := r.Read(buf)
	if err != nil || n != len(s) {
		return fmt.Errorf("expected %q, read failed: read %d byte(s), err: %v", s, n, err)
	}

	if string(buf) != s {
		return fmt.Errorf("expected %q, read %q", s, string(buf))
	}

	return nil
}

func parseHeader(r io.Reader) (*Header, error) {
	if err := readLiteralExpecting(r, "MThd"); err != nil {
		return nil, err
	}

	headerData, err := parseSizedChunk(r)
	if err != nil {
		return nil, err
	}

	rv := Header{}

	if err := binary.Read(bytes.NewBuffer(headerData), binary.BigEndian, &rv); err != nil {
		return nil, err
	}

	return &rv, nil
}

func readSizedChunk(r io.Reader) (io.Reader, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	return limitreader.New(r, int64(length)), nil
}

func skipSizedChunk(r io.Reader) error {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return err
	}

	return onReadBytes(r, int64(length), nil)
}

func onReadBytes(r io.Reader, n int64, f func([]byte) error) error {
	remaining := n
	bufSz := 4096
	buf := make([]byte, bufSz)

	for remaining > 0 {
		n := len(buf)
		if int64(n) > remaining {
			n = int(remaining)
		}

		if _, err := r.Read(buf[:n]); err != nil {
			return fmt.Errorf("read error: %v", err)
		}

		if f != nil {
			if err := f(buf[:n]); err != nil {
				return err
			}
		}

		remaining -= int64(n)
	}

	return nil
}

func parseSizedChunk(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	buf := make([]byte, length)
	n, err := r.Read(buf)
	if err != nil || n != int(length) {
		return nil, fmt.Errorf("expected chunk of length %d, read failed: read %d byte(s), err: %v", length, n, err)
	}

	return buf, nil
}

var (
	noData = errors.New("no data")
)

func readVarint(r io.Reader) (uint64, error) {
	buf := make([]byte, 1)
	var rv uint64

	for {
		if _, err := r.Read(buf); err != nil {
			if err == io.EOF && rv != 0 {
				return 0, errors.New("EOF in the middle of varint")
			}
			return 0, err
		}

		b := buf[0]

		rv |= uint64(b & 0x7f)
		continued := (b & 0x80) != 0
		if !continued {
			break
		}
		rv = rv << 7
	}

	return rv, nil
}

func parseVarint(data []byte) (uint64, int, error) {
	var rv uint64

	for i, b := range data {
		rv |= uint64(b & 0x7f)
		continued := (b & 0x80) != 0
		if !continued {
			return rv, i + 1, nil
		}
		rv = rv << 7
	}

	return 0, len(data), fmt.Errorf("error parsing varint: exhausted %d byte(s)", len(data))
}

/*
func parseVarint(r io.Reader) (uint64, error) {
	var b uint8
	var rv uint64

	initialByte := true

	for {
		if err := binary.Read(r, binary.BigEndian, &b); err != nil {
			if initialByte {
				return 0, noData
			}

			return 0, err
		}

		initialByte = false

		rv |= uint64(b & 0x7f)
		continued := (b & 0x80) != 0
		if !continued {
			return rv, nil
		}
		rv = rv << 7
	}
}
*/

func readUntil(r io.Reader, sentinel uint8) ([]byte, error) {
	var rv []byte
	for {
		var b byte
		if err := binary.Read(r, binary.BigEndian, &b); err != nil {
			return nil, err
		}

		if b == sentinel {
			return rv, nil
		}

		rv = append(rv, b)
	}
}

func parseTrack(r io.Reader) (*Track, bool, error) {
	if err := readLiteralExpecting(r, "MTrk"); err != nil {
		if err := skipSizedChunk(r); err != nil {
			return nil, false, err
		}
		return nil, false, err
	}

	rv := &Track{}

	trackReader, err := readSizedChunk(r)
	if err != nil {
		return nil, true, err
	}

	rawEvents, err := parseTrackBody(trackReader)
	if err != nil {
		return nil, true, err
	}

	for _, evt := range rawEvents {
		presentable, err := presentEvent(evt)
		if err != nil {
			return nil, true, err
		}

		rv.Events = append(rv.Events, presentable)
	}

	// Throw away events returned from parseTrackBody!
	return rv, true, nil
}

func (f *File) OnEvents(trackNo int, callback func(float64, Event) error) error {
	if f.Header.Division < 0 {
		return fmt.Errorf("SMPTE divisions (%v) are unimplemented (TODO)", f.Header.Division)
	}

	ticksPerBeat := f.Header.Division
	microsPerBeat := DefaultTempo

	if trackNo < 0 || trackNo >= len(f.Tracks) {
		return fmt.Errorf("no such track: %d (there are %d tracks)", trackNo, len(f.Tracks))
	}

	track := f.Tracks[trackNo]

	var seconds float64

	for i, evt := range track.Events {
		if err := callback(seconds, evt); err != nil {
			return fmt.Errorf("error handling event #%d at %fs: %v", i, seconds, err)
		}

		switch v := evt.(type) {
		case TimeDeltaEvent:
			ticksTaken := float64(v)
			beatsTaken := ticksTaken / float64(ticksPerBeat)
			microsTaken := beatsTaken * float64(microsPerBeat)
			secsTaken := microsTaken / 1e6
			seconds += secsTaken

		case MetaEvent:
			newTempo, ok := v.GetTempo()
			if ok {
				microsPerBeat = newTempo
			}

		default:
			// Do nothing
		}
	}

	return nil
}

func parseTrackBody(r io.Reader) ([]event, error) {
	parser := &eventDataParser{}

	for {
		timeDelta, err := readVarint(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading time-delta: %v", err)
		}

		if VeryDetailedLogging {
			log.Printf("parseTime: %d", timeDelta)
		}

		if timeDelta > 0 {
			parser.addTimeDelta(timeDelta)
		}

		err = parser.readSingleEvent(r)
		if err != nil {
			return nil, fmt.Errorf("error parsing event: %v", err)
		}

		if VeryDetailedLogging {
			log.Printf("parseEvent: %v", parser.events[len(parser.events)-1])
		}
	}

	return parser.events, nil
}

func parse(r io.Reader, strict bool) (*File, error) {
	hdr, err := parseHeader(r)
	if err != nil {
		return nil, fmt.Errorf("error parsing header: %v", err)
	}

	rv := &File{Header: hdr}

	sawMidiTrack := false
	var sawNonMidiTrack error

	for i := 0; i < int(hdr.NumberOfTracks); i++ {
		trk, wasMidiTrack, err := parseTrack(r)
		if !wasMidiTrack {
			if strict {
				return nil, fmt.Errorf("saw non-MIDI track: %v", err)
			}
			if sawNonMidiTrack == nil {
				sawNonMidiTrack = err
			}
			// We must skip unknown kinds of tracks.
			continue
		}
		sawMidiTrack = true
		if err != nil {
			return nil, fmt.Errorf("error parsing track %d: %v", i, err)
		}

		rv.Tracks = append(rv.Tracks, trk)
	}

	if !sawMidiTrack {
		return nil, fmt.Errorf("no MIDI tracks found: first track error: %v", sawNonMidiTrack)
	}

	return rv, nil
}

func Parse(r io.Reader) (*File, error) {
	ctxR := contextreader.New(r)

	f, err := parse(ctxR, false)
	if err != nil {
		return nil, ctxR.WrapError(err)
	}

	return f, nil
}
