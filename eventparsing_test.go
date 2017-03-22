package midi

import (
	"reflect"
	"testing"
)

type parseEventsCase struct {
	data []byte
	want []event
}

func TestParseEvents(t *testing.T) {
	testcases := []parseEventsCase{
		{
			[]byte("\x90\x3C\x7F"),
			[]event{
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
			},
		},
		{
			[]byte("\xFF\x05\x0a0123456789"),
			[]event{
				{metaEvent, 0x05, []byte("0123456789")},
			},
		},
		{
			[]byte("\xF7helloworld\xF7"),
			[]event{
				{sysexEvent, 0xF7, []byte("helloworld")},
			},
		},
		{
			[]byte("\xF0helloworld\xF7"),
			[]event{
				{sysexEvent, 0xF0, []byte("helloworld")},
			},
		},
		{
			[]byte("\x90\x3C\x7F\x90\x3C\x7F"),
			[]event{
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
			},
		},
		{
			[]byte("\x90\x3C\x7F\x3C\x7F"),
			[]event{
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
			},
		},
		{
			[]byte("\x90\x3C\x7F\xFF\x05\x0Ahelloworld\x90\x3C\x7F"),
			[]event{
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
				{metaEvent, 0x05, []byte("helloworld")},
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
			},
		},
		{
			[]byte("\x90\x3C\xF7blah\xF7\x7F"),
			[]event{
				{sysexEvent, 0xF7, []byte("blah")},
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
			},
		},
		{
			[]byte("\xFF\x05\x02\xF0blah\xF7\x42"),
			[]event{
				{sysexEvent, 0xF0, []byte("blah")},
				{metaEvent, 0x05, []byte{0xF0, 0x42}},
			},
		},
		{
			[]byte("\x90\x3C\x7F\x90\x40\x7F\x90\x43\x7F"),
			[]event{
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
				{midiEvent, 0x90, []byte{0x40, 0x7F}},
				{midiEvent, 0x90, []byte{0x43, 0x7F}},
			},
		},
		{
			[]byte("\x90\x3C\x7F\x40\x7F\x43\x7F"),
			[]event{
				{midiEvent, 0x90, []byte{0x3C, 0x7F}},
				{midiEvent, 0x90, []byte{0x40, 0x7F}},
				{midiEvent, 0x90, []byte{0x43, 0x7F}},
			},
		},
		{
			append([]byte("\xFF\x05\x81\x48"), make([]byte, 200)...),
			[]event{
				{metaEvent, 0x05, make([]byte, 200)},
			},
		},
	}
	n := len(testcases)

	for i, testcase := range testcases {
		parser := &eventDataParser{}
		if err := parser.feed(testcase.data); err != nil {
			t.Errorf("[%d/%d] parser.feed(%v) = err: %v", i+1, n, testcase.data, err)
			continue
		}

		if err := parser.finish(); err != nil {
			t.Errorf("[%d/%d] parser.feed(%v); parser.finish() = err: %v", i+1, n, testcase.data, err)
			continue
		}

		if !reflect.DeepEqual(parser.events, testcase.want) {
			t.Errorf("[%d/%d] parser.feed(%v); events = %v want %v", i+1, n, testcase.data, parser.events, testcase.want)
		}
	}
}
