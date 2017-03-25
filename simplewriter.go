package midi

import "io"

type SimpleWriter struct {
	divisions int16
	events    []Event
}

func NewSimpleWriter(divisions int16) *SimpleWriter {
	return &SimpleWriter{divisions: divisions}
}

func (s *SimpleWriter) Play(keys []int, velocity int, duration int) {
	for _, key := range keys {
		s.events = append(s.events, MIDIEvent{
			Type:     NoteOn,
			Key:      key,
			Velocity: velocity,
		})
	}
	s.TimeDelta(duration)
	for _, key := range keys {
		s.events = append(s.events, MIDIEvent{
			Type:     NoteOff,
			Key:      key,
			Velocity: velocity,
		})
	}
}

func (s *SimpleWriter) TimeDelta(duration int) {
	s.events = append(s.events, TimeDeltaEvent(duration))
}

func (s *SimpleWriter) Write(w io.Writer) error {
	f := File{
		Header: &Header{
			Format:         0,
			NumberOfTracks: 1,
			Division:       s.divisions,
		},
		Tracks: []*Track{
			&Track{
				Events: s.events,
			},
		},
	}
	data, err := f.encode()
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}
