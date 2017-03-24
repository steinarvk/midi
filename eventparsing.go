package midi

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type eventType int

const (
	midiEvent      eventType = iota
	sysexEvent     eventType = iota
	metaEvent      eventType = iota
	timeDeltaEvent eventType = iota
)

type parserState int

const (
	wantEvent       parserState = iota
	wantMetaType    parserState = iota
	wantMetaLength  parserState = iota
	wantMetaData    parserState = iota
	wantSysexLength parserState = iota
	wantSysexData   parserState = iota
)

type event struct {
	kind      eventType
	typeByte  byte
	data      []byte
	timeDelta uint64
}

var (
	metaEventNames = map[int]string{
		0x00: "SequenceNumber",
		0x01: "TextEvent",
		0x02: "CopyrightNotice",
		0x03: "TrackName",
		0x04: "InstrumentName",
		0x05: "LyricText",
		0x06: "MarkerText",
		0x07: "CuePoint",
		0x20: "ChannelPrefixAssignment",
		0x2F: "EndOfTrack",
		0x51: "TempoSetting",
		0x54: "SMPTEOffset",
		0x58: "TimeSignature",
		0x59: "KeySignature",
		0x7F: "SequencerSpecificEvent",
	}
)

func (e event) String() string {
	switch e.kind {
	case midiEvent:
		spec, present := midiEventSpecs[int((e.typeByte&0xf0)>>4)]
		var desc string
		if present {
			desc = spec.name
		} else {
			desc = fmt.Sprintf("Unknown:%02x", e.typeByte)
		}
		return fmt.Sprintf("MIDI %s % 02x", desc, e.data)

	case metaEvent:
		name, ok := metaEventNames[int(e.typeByte)]
		if !ok {
			name = fmt.Sprintf("Unknown:%02x", e.typeByte)
		}

		isText := strings.HasSuffix(name, "Text") || strings.HasSuffix(name, "Name")

		if isText {
			return fmt.Sprintf("Meta %s %q", name, string(e.data))
		}

		return fmt.Sprintf("Meta %s % 02x", name, e.data)

	case sysexEvent:
		return fmt.Sprintf("SysEx %02x % 02x", e.typeByte, e.data)

	case timeDeltaEvent:
		return fmt.Sprintf("Time += %d", e.timeDelta)

	default:
		return "<invalid>"
	}
}

type eventDataParser struct {
	state parserState

	bytesFed int64

	runningStatus byte

	eventData []byte

	metaEventType byte
	metaLength    int

	currentSysexType byte
	sysexLength      int
	sysexData        []byte

	events []event
}

func (p *eventDataParser) feed(data []byte) error {
	for _, x := range data {
		if _, err := p.feedByte(x); err != nil {
			return err
		}
	}
	return nil
}

func (p *eventDataParser) readSingleEvent(r io.Reader) error {
	buf := make([]byte, 1)
	done := false
	beganReading := false

	for !done {
		if _, err := r.Read(buf); err != nil {
			if err == io.EOF && beganReading {
				return errors.New("EOF in the middle of event")
			}
			return err
		}

		beganReading = true

		var err error
		done, err = p.feedByte(buf[0])
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *eventDataParser) feedSingleEvent(data []byte) (int, error) {
	consumed := 0
	for _, x := range data {
		consumed++

		done, err := p.feedByte(x)
		if err != nil {
			return consumed, err
		}

		if done {
			break
		}
	}

	return consumed, nil
}

func (p *eventDataParser) finish() error {
	if p.state != wantEvent {
		return fmt.Errorf("parser in unexpected state on EOF (%v)", p.state)
	}

	if p.currentSysexType != 0 {
		return fmt.Errorf("parser in unexpected sysex state (%02x) on EOF (%v)", p.currentSysexType)
	}

	if len(p.eventData) > 0 {
		return fmt.Errorf("parser has unflushed event data on EOF (data: %v)", p.eventData)
	}

	if len(p.sysexData) > 0 {
		return fmt.Errorf("parser has unflushed sysex data on EOF (data: %v)", p.sysexData)
	}

	return nil
}

func (p *eventDataParser) addTimeDelta(dt uint64) {
	p.events = append(p.events, event{
		kind:      timeDeltaEvent,
		timeDelta: dt,
	})
}

type midiEventSpec struct {
	name    string
	dataLen int
}

var (
	midiEventSpecs = map[int]midiEventSpec{
		0x8: midiEventSpec{name: "NoteOff", dataLen: 2},
		0x9: midiEventSpec{name: "NoteOn", dataLen: 2},
		0xA: midiEventSpec{name: "Aftertouch", dataLen: 2},
		0xB: midiEventSpec{name: "ControllerChange/ChannelMode", dataLen: 2},
		0xC: midiEventSpec{name: "ProgramChange", dataLen: 1},
		0xD: midiEventSpec{name: "ChannelKeyPressure", dataLen: 1},
		0xE: midiEventSpec{name: "PitchBend", dataLen: 2},
	}
)

func (p *eventDataParser) feedMIDIDataByte(data byte) (bool, error) {
	nibble := int((p.runningStatus & 0xf0) >> 4)
	spec, present := midiEventSpecs[nibble]
	if !present {
		return false, fmt.Errorf("no MIDI event spec for running status %02x", p.runningStatus)
	}

	p.eventData = append(p.eventData, data)

	switch {
	case len(p.eventData) == spec.dataLen:
		p.events = append(p.events, event{
			kind:     midiEvent,
			typeByte: p.runningStatus,
			data:     p.eventData,
		})
		p.eventData = nil
		return true, nil

	case len(p.eventData) >= spec.dataLen:
		return false, fmt.Errorf("MIDI event %02x already too long for expected length %d (data: %v)", p.runningStatus, spec.dataLen, p.eventData)
	}

	return false, nil
}

func (p *eventDataParser) feedByte(data byte) (bool, error) {
	stopPoint, err := p.feedByteInternal(data)
	if err != nil {
		return stopPoint, fmt.Errorf("after consuming %d event byte(s): %v", p.bytesFed, err)
	}

	p.bytesFed++
	return stopPoint, err
}

func (p *eventDataParser) feedByteInternal(data byte) (bool, error) {
	switch p.state {
	case wantEvent:
		if data == 0xFF {
			p.state = wantMetaType
			return false, nil
		}

		if data == 0xF0 || data == 0xF7 {
			p.currentSysexType = data
			p.state = wantSysexLength
			p.sysexLength = 0
			p.sysexData = nil
			return false, nil
		}

		if data&0x80 != 0 {
			if len(p.eventData) > 0 {
				return false, fmt.Errorf("running status changed (from %02x to %02x) in the middle of event (partial data: %02x)", p.runningStatus, data, p.eventData)
			}
			p.runningStatus = data
			return false, nil
		}

		return p.feedMIDIDataByte(data)

	case wantMetaType:
		p.metaEventType = data
		p.state = wantMetaLength
		p.metaLength = 0
		p.eventData = nil

	case wantSysexLength:
		p.sysexLength = (p.sysexLength << 7) | (int(data) & 0x7F)
		if data&0x80 == 0 {
			p.state = wantSysexData
		}

		if p.sysexLength == 0 {
			p.flushSysexEvent()
			return true, nil
		}

	case wantSysexData:
		p.sysexData = append(p.sysexData, data)
		if len(p.sysexData) == p.sysexLength {
			p.flushSysexEvent()
			return true, nil
		}

	case wantMetaLength:
		p.metaLength = (p.metaLength << 7) | (int(data) & 0x7F)
		if data&0x80 == 0 {
			p.state = wantMetaData
		}

		if p.metaLength == 0 {
			p.events = append(p.events, event{
				kind:     metaEvent,
				typeByte: p.metaEventType,
				data:     p.eventData,
			})
			p.state = wantEvent
			p.eventData = nil

			return true, nil
		}

	case wantMetaData:
		p.eventData = append(p.eventData, data)

		if len(p.eventData) == p.metaLength {
			p.events = append(p.events, event{
				kind:     metaEvent,
				typeByte: p.metaEventType,
				data:     p.eventData,
			})
			p.state = wantEvent
			p.eventData = nil

			return true, nil
		}

	default:
		panic(fmt.Errorf("parser in illegal state: %v", p.state))
	}

	return false, nil
}

func (p *eventDataParser) flushSysexEvent() {
	p.events = append(p.events, event{
		kind:     sysexEvent,
		typeByte: p.currentSysexType,
		data:     p.sysexData,
	})
	p.state = wantEvent
	p.sysexData = nil
	p.currentSysexType = 0
}
