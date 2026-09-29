package ui

import (
	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/palette"
)

// line is one line of the list as it is drawn: a row of the ranked list, or
// the heading of a group, which no selection lands on.
type line struct {
	row     int
	heading string
}

func (l line) isHeading() bool { return l.row < 0 }

// The groups an empty query lays the list out in, in the order they are
// drawn. What needs you comes first: an agent that is waiting or has finished
// is what the palette is most often opened to reach.
const (
	groupNeedsYou = iota
	groupRecent
	groupCommands
	groupOpen
	groupCount
)

var groupHeadings = [groupCount]string{"NEEDS YOU", "RECENT", "COMMANDS", "OPEN"}

func groupOf(ranked palette.Ranked) int {
	status := herdr.AgentStatus(ranked.Status)
	switch {
	case ranked.Entry.Goes && (status == herdr.AgentStatusBlocked || status == herdr.AgentStatusDone):
		return groupNeedsYou
	case ranked.Score > 0:
		// An empty query scores nothing but how recently a row ran.
		return groupRecent
	case !ranked.Entry.Goes:
		return groupCommands
	}
	return groupOpen
}

// grouped lays out the list an empty query ranked, each group in the order
// the ranking gave it. A list that falls into one group has nothing to tell
// apart, so it goes without headings.
func grouped(ranked []palette.Ranked) ([]palette.Ranked, []line) {
	var groups [groupCount][]palette.Ranked
	for _, r := range ranked {
		group := groupOf(r)
		groups[group] = append(groups[group], r)
	}
	// Blocked before done: a blocked agent cannot go on until it is answered.
	needs := groups[groupNeedsYou]
	blocked := make([]palette.Ranked, 0, len(needs))
	var rest []palette.Ranked
	for _, r := range needs {
		if herdr.AgentStatus(r.Status) == herdr.AgentStatusBlocked {
			blocked = append(blocked, r)
		} else {
			rest = append(rest, r)
		}
	}
	groups[groupNeedsYou] = append(blocked, rest...)

	filled := 0
	for _, rows := range groups {
		if len(rows) > 0 {
			filled++
		}
	}

	out := make([]palette.Ranked, 0, len(ranked))
	lines := make([]line, 0, len(ranked)+groupCount)
	for group, rows := range groups {
		if len(rows) == 0 {
			continue
		}
		if filled > 1 {
			lines = append(lines, line{row: -1, heading: groupHeadings[group]})
		}
		for _, r := range rows {
			lines = append(lines, line{row: len(out)})
			out = append(out, r)
		}
	}
	return out, lines
}

// ungrouped is a line per row, which is how a query's matches are drawn.
func ungrouped(rows int) []line {
	lines := make([]line, rows)
	for i := range lines {
		lines[i] = line{row: i}
	}
	return lines
}
