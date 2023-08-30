package log

import (
	"flag"
	"log"
)

var (
	debug = flag.Bool("debug", false, "Debug mode.")
)

func Debugf(format string, args ...any) {
	if *debug {
		log.Printf(format, args...)
	}
}
