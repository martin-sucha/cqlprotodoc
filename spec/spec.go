// Package spec implements parser for Cassandra protocol specification.
package spec

import (
	"fmt"
	"regexp"
	"strings"
)

type Document struct {
	License  string
	Title    string
	TOC      []TOCEntry
	Sections []Section
}

type TOCEntry struct {
	Number string
	Title  string
}

type Section struct {
	Number string
	Title  string
	Body   string
}

var commentRegexp = regexp.MustCompile("^# ?(.*)$")
var emptyRegexp = regexp.MustCompile(`^\s*$`)
var titleRegexp = regexp.MustCompile(`^\s+(.*)\s*$`)
var headingRegexp = regexp.MustCompile(`^(\s*)(\d+(?:\.\d+)*)\.? (.*)$`)

const (
	mhSpaces = 1
	mhNumber = 2
	mhTitle  = 3
)

func Parse(data string) (Document, error) {
	lines := strings.Split(data, "\n")
	var license strings.Builder
	var doc Document
	l := 0
	// license
	for l < len(lines) {
		m := commentRegexp.FindStringSubmatch(lines[l])
		if len(m) != 2 {
			break
		}
		license.WriteString(m[1])
		license.WriteString("\n")
		l++
	}
	doc.License = license.String()
	// empty lines
	for l < len(lines) && emptyRegexp.MatchString(lines[l]) {
		l++
	}
	// title
	if l >= len(lines) {
		return Document{}, fmt.Errorf("missing title")
	}
	m := titleRegexp.FindStringSubmatch(lines[l])
	if len(m) != 2 {
		return Document{}, fmt.Errorf("line %d: title expected on line", l)
	}
	doc.Title = m[1]
	l++
	// empty lines
	for l < len(lines) && emptyRegexp.MatchString(lines[l]) {
		l++
	}
	// table of contents header
	if lines[l] != "Table of Contents" {
		return Document{}, fmt.Errorf("line %d: expected table of contents", l)
	}
	l++
	// empty lines
	for l < len(lines) && emptyRegexp.MatchString(lines[l]) {
		l++
	}
	// toc entries
	for l < len(lines) {
		if emptyRegexp.MatchString(lines[l]) {
			// end of toc
			break
		}
		mh := headingRegexp.FindStringSubmatch(lines[l])
		if len(mh) != 4 {
			return Document{}, fmt.Errorf("line %d: expected toc entry", l)
		}
		doc.TOC = append(doc.TOC, TOCEntry{
			Number: mh[mhNumber],
			Title:  mh[mhTitle],
		})
		l++
	}
	// empty lines
	for l < len(lines) && emptyRegexp.MatchString(lines[l]) {
		l++
	}
	// content
	tocIdx := 0
	var section Section
	var body []string

	for l < len(lines) {
		var sectionStart bool
		var newSection Section
		sectionStart, tocIdx, newSection = checkSectionStart(doc.TOC, tocIdx, lines[l])
		if sectionStart {
			section.Body = strings.Join(body, "\n")
			doc.Sections = append(doc.Sections, section)
			section = newSection
			body = nil
			l++
			// Eat empty lines
			for l < len(lines) && emptyRegexp.MatchString(lines[l]) {
				l++
			}
			continue
		}
		body = append(body, lines[l])
		l++
	}

	var emptySection Section
	if len(body) > 0 || section != emptySection {
		section.Body = strings.Join(body, "\n")
		doc.Sections = append(doc.Sections, section)
	}

	return doc, nil
}

// checkSectionStart checks if the line starts a new section and returns a new tocIdx.
func checkSectionStart(toc []TOCEntry, tocIdx int, line string) (bool, int, Section) {
	mh := headingRegexp.FindStringSubmatch(line)
	if len(mh) != 4 || tocIdx >= len(toc) {
		return false, tocIdx, Section{}
	}

	if mh[mhSpaces] == "" {
		if mh[mhNumber] == toc[tocIdx].Number {
			tocIdx++
		}
		return true, tocIdx, Section{
			Number: mh[mhNumber],
			Title:  mh[mhTitle],
		}
	}

	t := strings.ToLower(mh[3])
	for i := tocIdx; i < len(toc); i++ {
		t2 := strings.ToLower(toc[i].Title)
		if mh[mhNumber] == toc[i].Number && (strings.Contains(t, t2) || strings.Contains(t2, t)) {
			return true, i + 1, Section{
				Number: mh[mhNumber],
				Title:  mh[mhTitle],
			}
		}
	}

	return false, tocIdx, Section{}
}
