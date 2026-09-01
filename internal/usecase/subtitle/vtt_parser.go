package subtitle

import (
	"fmt"
	"strconv"
	"strings"
)

// vttCue is one parsed WebVTT cue: a time range plus its (possibly
// multi-line, here flattened to one line) caption text.
type vttCue struct {
	StartSec float64
	EndSec   float64
	Text     string
}

// parseVTT parses a standard WebVTT document (the "WEBVTT" header, optional
// cue identifiers, "start --> end" timing lines, and text lines separated by
// blank lines) into an ordered list of cues. It's intentionally lenient
// (ignores cue settings like "align:start", tolerates a missing/blank
// header) since real-world .vtt files vary in exactly how strict they are.
func parseVTT(content string) ([]vttCue, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	blocks := strings.Split(strings.TrimSpace(content), "\n\n")

	cues := make([]vttCue, 0, len(blocks))
	for _, block := range blocks {
		lines := splitNonEmptyLines(block)
		if len(lines) == 0 {
			continue
		}

		// Skip a leading "WEBVTT" header block (with optional metadata lines).
		if strings.HasPrefix(strings.ToUpper(lines[0]), "WEBVTT") {
			continue
		}

		timingLineIdx := -1
		for i, l := range lines {
			if strings.Contains(l, "-->") {
				timingLineIdx = i
				break
			}
		}
		if timingLineIdx == -1 {
			// No timing line in this block (e.g. a stray identifier or note) — skip it.
			continue
		}

		start, end, err := parseTimingLine(lines[timingLineIdx])
		if err != nil {
			return nil, fmt.Errorf("invalid WebVTT timing line %q: %w", lines[timingLineIdx], err)
		}

		text := strings.TrimSpace(strings.Join(lines[timingLineIdx+1:], " "))
		cues = append(cues, vttCue{StartSec: start, EndSec: end, Text: text})
	}

	return cues, nil
}

func splitNonEmptyLines(block string) []string {
	rawLines := strings.Split(block, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, l := range rawLines {
		if trimmed := strings.TrimSpace(l); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// parseTimingLine parses "00:00:01.000 --> 00:00:04.000 [cue settings...]".
func parseTimingLine(line string) (float64, float64, error) {
	parts := strings.SplitN(line, "-->", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("missing '-->' separator")
	}

	start, err := parseTimestamp(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("start timestamp: %w", err)
	}

	// The end side may have trailing cue settings, e.g. "00:00:04.000 align:start".
	endField := strings.Fields(strings.TrimSpace(parts[1]))
	if len(endField) == 0 {
		return 0, 0, fmt.Errorf("missing end timestamp")
	}
	end, err := parseTimestamp(endField[0])
	if err != nil {
		return 0, 0, fmt.Errorf("end timestamp: %w", err)
	}

	return start, end, nil
}

// parseTimestamp parses "HH:MM:SS.mmm" or the shorter "MM:SS.mmm" form (both
// valid in WebVTT), with either '.' or ',' as the fractional separator.
func parseTimestamp(ts string) (float64, error) {
	ts = strings.ReplaceAll(ts, ",", ".")

	segs := strings.Split(ts, ":")
	var hours, minutes int
	var secondsStr string

	switch len(segs) {
	case 3:
		h, err := strconv.Atoi(segs[0])
		if err != nil {
			return 0, err
		}
		m, err := strconv.Atoi(segs[1])
		if err != nil {
			return 0, err
		}
		hours, minutes, secondsStr = h, m, segs[2]
	case 2:
		m, err := strconv.Atoi(segs[0])
		if err != nil {
			return 0, err
		}
		minutes, secondsStr = m, segs[1]
	default:
		return 0, fmt.Errorf("expected HH:MM:SS.mmm or MM:SS.mmm, got %q", ts)
	}

	seconds, err := strconv.ParseFloat(secondsStr, 64)
	if err != nil {
		return 0, err
	}

	return float64(hours*3600+minutes*60) + seconds, nil
}
