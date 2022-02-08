package main

import (
	"cqlprotodoc/spec"
	"embed"
	"fmt"
	"github.com/mvdan/xurls"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed template.gohtml
var templateFS embed.FS

type TOCNode struct {
	spec.TOCEntry
	Children []*TOCNode
}

func buildTOCTree(entries []spec.TOCEntry) []*TOCNode {
	var root TOCNode
	stack := []*TOCNode{&root}
	for _, e := range entries {
		level := strings.Count(e.Number, ".") + 1
		if len(stack) > level {
			stack = stack[:level]
		}
		parent := stack[len(stack)-1]
		node := &TOCNode{
			TOCEntry: e,
			Children: nil,
		}
		parent.Children = append(parent.Children, node)
		stack = append(stack, node)
	}
	return root.Children
}

type templateData struct {
	spec.Document
	TOCTree  []*TOCNode
	Sections []Section
}

type Section struct {
	spec.Section
	Level    int
	BodyHTML template.HTML
}

var linkifyRegexp *regexp.Regexp
var sectionSubexpIdx int
var sectionsSubexpIdx int

func init() {
	s := xurls.Strict.String()
	r := `(?:<URL>)|[Ss]ection (\d+(?:\.\d+)*)|[Ss]ections (\d+(?:\.\d+)*(?:(?:, (?:and )?| and )\d+(?:\.\d+)*)*)`
	linkifyRegexp = regexp.MustCompile(strings.ReplaceAll(r, "<URL>", s))
	sectionSubexpIdx = xurls.Strict.NumSubexp()*2 + 2
	sectionsSubexpIdx = (xurls.Strict.NumSubexp()+1)*2 + 2
}

var sectionsSplitRegexp = regexp.MustCompile("(?:, (?:and )?| and )")

func link(sb *strings.Builder, href, text string) {
	sb.WriteString(`<a href="`)
	sb.WriteString(template.HTMLEscapeString(href))
	sb.WriteString(`">`)
	sb.WriteString(template.HTMLEscapeString(text))
	sb.WriteString(`</a>`)
}

func formatBody(s string) template.HTML {
	var sb strings.Builder
	lastIdx := 0
	for _, m := range linkifyRegexp.FindAllStringSubmatchIndex(s, -1) {
		sb.WriteString(template.HTMLEscapeString(s[lastIdx:m[0]]))

		switch {
		case m[sectionSubexpIdx] != -1:
			sectionNo := s[m[sectionSubexpIdx]:m[sectionSubexpIdx+1]]
			link(&sb, "#s"+sectionNo, s[m[0]:m[1]])
		case m[sectionsSubexpIdx] != -1:
			sb.WriteString(s[m[0]:m[sectionsSubexpIdx]])
			sections := s[m[sectionsSubexpIdx]:m[sectionsSubexpIdx+1]]
			lastIdx2 := 0
			for _, m2 := range sectionsSplitRegexp.FindAllStringIndex(sections, -1) {
				sectionNo := sections[lastIdx2:m2[0]]
				link(&sb, "#s"+sectionNo, sectionNo)
				// separator
				sb.WriteString(sections[m2[0]:m2[1]])
				lastIdx2 = m2[1]
			}
			sectionNo := sections[lastIdx2:]
			link(&sb, "#s"+sectionNo, sectionNo)
		default:
			href := s[m[0]:m[1]]
			link(&sb, href, href)
		}
		lastIdx = m[1]
	}
	sb.WriteString(template.HTMLEscapeString(s[lastIdx:]))
	return template.HTML(sb.String())
}

func buildSections(in []spec.Section) []Section {
	ret := make([]Section, len(in))
	for i, s := range in {
		ret[i].Section = in[i]
		ret[i].Level = strings.Count(s.Number, ".") + 2
		ret[i].BodyHTML = formatBody(in[i].Body)
	}
	return ret
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: <cassandra_doc_dir> <output_dir>")
		return
	}
	inputDir := os.Args[1]
	outputDir := os.Args[2]
	data, err := os.ReadFile(filepath.Join(inputDir, "native_protocol_v5.spec"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	doc, err := spec.Parse(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	tmpl, err := template.ParseFS(templateFS, "template.gohtml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	f, err := os.Create(filepath.Join(outputDir, "native_protocol_v5.html"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	err = tmpl.Execute(f, templateData{
		Document: doc,
		TOCTree:  buildTOCTree(doc.TOC),
		Sections: buildSections(doc.Sections),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
}
