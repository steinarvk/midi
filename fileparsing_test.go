package midi

import (
	"bytes"
	"reflect"
	"testing"
)

func TestParseTrackBody(t *testing.T) {
	testcases := []parseEventsCase{
		{
			[]byte("\x00\x90\x3C\x7F"),
			[]event{
				{midiEvent, 0x90, []byte{0x3C, 0x7F}, 0},
			},
		},
		{
			[]byte("\x0a\x90\x3C\x7F"),
			[]event{
				{timeDeltaEvent, 0, nil, 10},
				{midiEvent, 0x90, []byte{0x3C, 0x7F}, 0},
			},
		},
		{
			[]byte("\x00\x90\x3C\x7F\x00\x3C\x7F"),
			[]event{
				{midiEvent, 0x90, []byte{0x3C, 0x7F}, 0},
				{midiEvent, 0x90, []byte{0x3C, 0x7F}, 0},
			},
		},
		{
			[]byte("\x00\xff\x2f\x00"),
			[]event{
				{metaEvent, 0x2f, nil, 0},
			},
		},
		{
			[]byte("\x00\xff\x51\x03\x05\xe3\x8b\xce\x40\xff\x2f\x00"),
			[]event{
				{metaEvent, 0x51, []byte{0x05, 0xe3, 0x8b}, 0},
				{timeDeltaEvent, 0, nil, 10048},
				{metaEvent, 0x2f, nil, 0},
			},
		},
	}

	n := len(testcases)

	for i, testcase := range testcases {
		events, err := parseTrackBody(bytes.NewBuffer(testcase.data))
		if err != nil {
			t.Errorf("[%d/%d] parseTrackBody(% 02x) = err: %v", i+1, n, testcase.data, err)
			continue
		}

		if !reflect.DeepEqual(events, testcase.want) {
			t.Errorf("[%d/%d] parseTrackBody(% 02x) = %v want %v", i+1, n, testcase.data, events, testcase.want)
		}
	}
}

func TestParseFullFile(t *testing.T) {
	data := append(
		[]byte("MThd\x00\x00\x00\x06\x00\x01\x00\x02\x00\xc0"),
		append(
			[]byte("MTrk\x00\x00\x00\x04\x00\xff\x2f\x00"),
			[]byte("MTrk\x00\x00\x00\x04\x0a\x90\x3C\x7F")...)...)

	_, err := parse(bytes.NewBuffer(data), true)
	if err != nil {
		t.Errorf("parse(%02x, true) = err: %v", data, err)
	}
}
