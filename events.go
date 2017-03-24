package midi

import (
	"fmt"
	"strings"
)

type TimeDeltaEvent int64
type SysexEvent []byte
type MetaEvent struct {
	Type byte
	Data []byte
}

type MIDIEventType byte

const (
	NoteOn           MIDIEventType = 0x90
	NoteOff          MIDIEventType = 0x80
	Aftertouch       MIDIEventType = 0xA0
	ControllerChange MIDIEventType = 0xB0
	ProgramChange    MIDIEventType = 0xC0
	ChannelPressure  MIDIEventType = 0xD0
	PitchBend        MIDIEventType = 0xE0
)

type MIDIEvent struct {
	Type MIDIEventType

	RawType byte

	Channel int

	Key      int
	Velocity int

	ControllerNumber int
	ControllerValue  int

	ProgramNumber int

	RawData []byte
}

func presentEvent(evt event) (interface{}, error) {
	switch evt.kind {
	case sysexEvent:
		return SysexEvent(append([]byte{evt.typeByte}, evt.data...)), nil

	case metaEvent:
		return MetaEvent{
			Type: evt.typeByte,
			Data: evt.data,
		}, nil

	case timeDeltaEvent:
		return TimeDeltaEvent(int64(evt.timeDelta)), nil

	case midiEvent:
		rv := MIDIEvent{
			Type:    MIDIEventType(0xf0 & evt.typeByte),
			RawType: evt.typeByte,
			Channel: int(0x0f & evt.typeByte),
			RawData: evt.data,
		}

		expectLen := func(n int) error {
			if len(rv.RawData) != n {
				return fmt.Errorf("%02x: want length %d, got %d (%v)", rv.Type, n, len(rv.RawData), rv.RawData)
			}
			return nil
		}

		switch rv.Type {
		case NoteOn:
			if err := expectLen(2); err != nil {
				return nil, err
			}
			rv.Key = int(rv.RawData[0])
			rv.Velocity = int(rv.RawData[1])

			if rv.Velocity == 0 {
				rv.Type = NoteOff
				rv.Velocity = 0x40
			}

		case NoteOff:
			if err := expectLen(2); err != nil {
				return nil, err
			}
			rv.Key = int(rv.RawData[0])
			rv.Velocity = int(rv.RawData[1])

		case Aftertouch:
			if err := expectLen(2); err != nil {
				return nil, err
			}
			rv.Key = int(rv.RawData[0])
			rv.Velocity = int(rv.RawData[1])

		case ControllerChange:
			if err := expectLen(2); err != nil {
				return nil, err
			}
			rv.ControllerNumber = int(rv.RawData[0])
			rv.ControllerValue = int(rv.RawData[1])

		case ProgramChange:
			if err := expectLen(1); err != nil {
				return nil, err
			}
			rv.ProgramNumber = int(rv.RawData[0])

		case ChannelPressure:
			if err := expectLen(1); err != nil {
				return nil, err
			}
			rv.Velocity = int(rv.RawData[0])

		case PitchBend:
			if err := expectLen(2); err != nil {
				return nil, err
			}
		}

		return rv, nil

	default:
		return nil, fmt.Errorf("invalid event %v: kind %v unknown", evt, evt.kind)
	}
}

func (e TimeDeltaEvent) String() string {
	return fmt.Sprintf("TimeDelta %d", int(e))
}

func (e SysexEvent) String() string {
	return fmt.Sprintf("SysEx %02x", []byte(e))
}

func (e MetaEvent) String() string {
	name, ok := metaEventNames[int(e.Type)]
	if !ok {
		name = fmt.Sprintf("Unknown:%02x", e.Type)
	}
	isText := strings.HasSuffix(name, "Text") || strings.HasSuffix(name, "Name") || strings.HasPrefix(name, "Text")

	if isText {
		return fmt.Sprintf("Meta %s %q", name, string(e.Data))
	}

	return fmt.Sprintf("Meta %s % 02x", name, e.Data)
}

func (e MIDIEvent) String() string {
	prefix := fmt.Sprintf("MIDI ch=%d ", e.Channel)

	switch e.Type {
	case NoteOn:
		return prefix + fmt.Sprintf("NoteOn k=%02x v=%02x", e.Key, e.Velocity)

	case NoteOff:
		return prefix + fmt.Sprintf("NoteOff k=%02x v=%02x", e.Key, e.Velocity)

	default:
		spec, present := midiEventSpecs[int(e.Type>>4)]
		var desc string
		if present {
			desc = spec.name
		} else {
			desc = fmt.Sprintf("Unknown:%02x", e.Type)
		}
		return prefix + fmt.Sprintf("%s % 02x", desc, e.RawData)
	}
}
