package main

import (
	"fmt"
	"strings"
	"time"
)

// parseDuration extends time.ParseDuration to support 'd' (days) and 'w'
// (weeks)
func parseDuration(s string) (time.Duration, error) {
	// Try standard parsing first
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	// Handle days (d) and weeks (w)
	if strings.HasSuffix(s, "d") {
		days := s[:len(s)-1]
		var n int
		_, err := fmt.Sscanf(days, "%d", &n)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}

	if strings.HasSuffix(s, "w") {
		weeks := s[:len(s)-1]
		var n int
		_, err := fmt.Sscanf(weeks, "%d", &n)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}

	return 0, fmt.Errorf("invalid duration: %s", s)
}
