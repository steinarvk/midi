package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/steinarvk/midi"
)

var (
	scanPath     = flag.String("path", "", "MIDI file scan path (dir or file)")
	logSuccesses = flag.Bool("log_success", false, "log individual parsing successes")
	showFiles    = flag.Bool("show_files", false, "log contents of tracks")
	showHeader   = flag.Bool("show_headers", false, "log headers of files")
	verbose      = flag.Bool("verbose", false, "very detailed logging")
)

func main() {
	flag.Parse()

	if *scanPath == "" {
		log.Fatalf("missing required argument: --path")
	}

	if *verbose {
		midi.VeryDetailedLogging = true
	}

	var successes, failures int64
	var successSize, totalSize int64

	onEachDir := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(strings.ToLower(path), ".mid") {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error opening %q: %v", path, err)
		}
		defer f.Close()

		totalSize += info.Size()

		data, err := midi.Parse(f)
		if err != nil {
			log.Printf("parsing %q: error: %v", path, err)
			failures++
		} else {
			if *logSuccesses {
				log.Printf("parsing %q: ok: %v", path, data)
			}
			if *showHeader {
				log.Printf("file %q header: format=%d tracks=%d division=%d", path, data.Header.Format, data.Header.NumberOfTracks, data.Header.Division)
			}
			if *showFiles {
				log.Printf("showing file %q", path)
				for i, track := range data.Tracks {
					for j, event := range track.Events {
						log.Printf("trk % 2d evt % 8d %v", i, j, event)
					}
				}
			}
			successes++
			successSize += info.Size()
		}

		return nil
	}

	t0 := time.Now()
	if err := filepath.Walk(*scanPath, onEachDir); err != nil {
		log.Fatalf("scanning failed: %v", err)
	}
	t1 := time.Now()

	secs := t1.Sub(t0).Seconds()

	ok := (successes == failures) && successes > 0
	pct := 100 * float64(successes) / float64(successes+failures)

	log.Printf("%d/%d file(s) parsed successfully", successes, failures+successes)
	log.Printf("%d files of a total of %d bytes parsed successfully", successes, successSize)
	log.Printf("Success rate: %.2f%%", pct)
	log.Printf("Time taken: %v", t1.Sub(t0))
	log.Printf("Successful bytes parsed per second: %v", float64(successSize)/secs)
	log.Printf("Total bytes parsed per second: %v", float64(totalSize)/secs)

	if ok {
		log.Fatalf("failures encountered")
	}
}
