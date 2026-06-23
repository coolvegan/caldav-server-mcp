package main

import (
	"fmt"
	"strings"
)

const FILEEXT = ".ics"

type Event struct {
	UID         string
	DTSTART     string
	DTEND       string
	SUMMARY     string
	LOCATION    string
	DESCRIPTION string
}

func parseEvent(raw string) (Event, error) {
	var e Event
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.SplitN(parts[0], ";", 2)[0]
		val := parts[1]
		switch name {
		case "UID":
			e.UID = val
		case "DTSTART":
			e.DTSTART = val
		case "DTEND":
			e.DTEND = val
		case "SUMMARY":
			e.SUMMARY = val
		case "LOCATION":
			e.LOCATION = val
		case "DESCRIPTION":
			e.DESCRIPTION = val
		}
	}
	if e.UID == "" {
		return e, fmt.Errorf("UID fehlt")
	}
	return e, nil
}
