package main

import (
	"flag"
	"log"
	"os"

	"github.com/steinarvk/midi"
)

var (
	output = flag.String("output", "", "MIDI file output")
)

func main() {
	flag.Parse()

	if *output == "" {
		log.Fatalf("missing required argument: --output")
	}

	f, err := os.Create(*output)
	if err != nil {
		log.Fatalf("error creating %q: %v", *output, err)
	}

	sw := midi.NewSimpleWriter()
	for i := 0; i < 10; i++ {
		sw.Play([]int{42 + i}, 0x40, 100)
		sw.TimeDelta(100)
	}

	if err := sw.Write(f); err != nil {
		log.Fatalf("error writing MIDI: %v", err)
	}

	if err := f.Close(); err != nil {
		log.Fatalf("error closing %q: %v", *output, err)
	}

}
