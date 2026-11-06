package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	schedulePath := flag.String("schedule", "", "path to schedule JSON file")
	overridesPath := flag.String("overrides", "", "path to overrides JSON file")
	fromFlag := flag.String("from", "", "RFC3339 timestamp to start listing entries from")
	untilFlag := flag.String("until", "", "RFC3339 timestamp to list entries until")
	flag.Parse()

	if *schedulePath == "" || *fromFlag == "" || *untilFlag == "" {
		fmt.Fprintln(os.Stderr, "error: --schedule, --from and --until are all required")
		os.Exit(1)
	}

	from, err := time.Parse(time.RFC3339, *fromFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing --from: %s\n", err)
		os.Exit(1)
	}

	until, err := time.Parse(time.RFC3339, *untilFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing --until: %s\n", err)
		os.Exit(1)
	}

	schedule, err := LoadSchedule(*schedulePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	// --overrides is optional. Not passing it at all means "no overrides", but
	// if it was passed and the file can't be read or parsed, that's a hard
	// failure, not something to silently fall back from, see notes.md.
	var overrides Entries
	if *overridesPath != "" {
		overrides, err = LoadOverrides(*overridesPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
	}

	entries, err := BuildSchedule(schedule, overrides, from, until)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	if entries == nil {
		entries = Entries{}
	}

	output, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshalling entries: %s\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}
