package midi

import "fmt"

type eventType int

const (
	midiEvent  eventType = iota
	sysexEvent eventType = iota
	metaEvent  eventType = iota
)

type parserState int

const (
	wantEvent      parserState = iota
	wantMetaType   parserState = iota
	wantMetaLength parserState = iota
	wantMetaData   parserState = iota
)

type event struct {
	kind     eventType
	typeByte byte
	data     []byte
}

type eventDataParser struct {
	state parserState

	bytesFed int64

	runningStatus byte

	eventData []byte

	metaEventType byte
	metaLength    int

	currentSysexType byte
	currentSysexData []byte

	events []event
}

func (p *eventDataParser) feed(data []byte) error {
	for _, x := range data {
		if err := p.feedByte(x); err != nil {
			return err
		}
	}
	return nil
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

	if len(p.currentSysexData) > 0 {
		return fmt.Errorf("parser has unflushed sysex data on EOF (data: %v)", p.currentSysexData)
	}

	return nil
}

func (p *eventDataParser) feedSysexByte(data byte) error {
	if data == 0xF7 {
		p.events = append(p.events, event{
			kind:     sysexEvent,
			typeByte: p.currentSysexType,
			data:     p.currentSysexData,
		})
		p.currentSysexType = 0
		p.currentSysexData = nil
	} else {
		p.currentSysexData = append(p.currentSysexData, data)
	}
	return nil
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

func (p *eventDataParser) feedMIDIDataByte(data byte) error {
	nibble := int((p.runningStatus & 0x80) >> 4)
	spec, present := midiEventSpecs[nibble]
	if !present {
		return fmt.Errorf("no MIDI event spec for running status %02x", p.runningStatus)
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

	case len(p.eventData) >= spec.dataLen:
		return fmt.Errorf("MIDI event %02x already too long for expected length %d (data: %v)", p.runningStatus, spec.dataLen, p.eventData)
	}

	return nil
}

func (p *eventDataParser) feedByte(data byte) error {
	err := p.feedByteInternal(data)
	if err != nil {
		return fmt.Errorf("after consuming %d event byte(s): %v", p.bytesFed, err)
	}

	p.bytesFed++
	return nil
}

func (p *eventDataParser) feedByteInternal(data byte) error {
	if p.currentSysexType != 0 {
		return p.feedSysexByte(data)
	}

	if data == 0xF0 || data == 0xF7 {
		p.currentSysexType = data
		if data == 0xF7 {
			return nil
		}
	}

	switch p.state {
	case wantEvent:
		if data == 0xFF {
			p.state = wantMetaType
			return nil
		}

		if data&0x80 != 0 {
			if len(p.eventData) > 0 {
				return fmt.Errorf("running status changed (from %02x to %02x) in the middle of event (partial data: %v)", p.runningStatus, data, p.eventData)
			}
			p.runningStatus = data
			return nil
		}

		return p.feedMIDIDataByte(data)

	case wantMetaType:
		p.metaEventType = data
		p.state = wantMetaLength
		p.metaLength = 0
		p.eventData = nil

	case wantMetaLength:
		p.metaLength = (p.metaLength << 7) | (int(data) & 0x7F)
		if data&0x80 == 0 {
			p.state = wantMetaData
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
		}

	default:
		panic(fmt.Errorf("parser in illegal state: %v", p.state))
	}

	return nil
}
