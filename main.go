package main

import (
	"cqlprotodoc/spec"
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

//go:embed template.gohtml
var templateFS embed.FS

type TOCNode struct {
	spec.TOCEntry
	Exists   bool
	Children []*TOCNode
}

func buildTOCTree(entries []spec.TOCEntry, sectionNumbers map[string]struct{}) []*TOCNode {
	var root TOCNode
	stack := []*TOCNode{&root}
	for _, e := range entries {
		level := strings.Count(e.Number, ".") + 1
		if len(stack) > level {
			stack = stack[:level]
		}
		parent := stack[len(stack)-1]
		_, exists := sectionNumbers[e.Number]
		node := &TOCNode{
			TOCEntry: e,
			Children: nil,
			Exists:   exists,
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

func link(sb *strings.Builder, href, text string) {
	sb.WriteString(`<a href="`)
	sb.WriteString(template.HTMLEscapeString(href))
	sb.WriteString(`">`)
	sb.WriteString(template.HTMLEscapeString(text))
	sb.WriteString(`</a>`)
}

func formatBody(text []spec.Text, sectionNumbers map[string]struct{}) template.HTML {
	var sb strings.Builder
	for _, t := range text {
		switch {
		case t.SectionRef != "":
			if _, ok := sectionNumbers[t.SectionRef]; ok {
				link(&sb, "#s"+t.SectionRef, t.Text)
			} else {
				sb.WriteString(template.HTMLEscapeString(t.Text))
			}
		case t.Href != "":
			link(&sb, t.Href, t.Text)
		default:
			sb.WriteString(template.HTMLEscapeString(t.Text))
		}
	}
	return template.HTML(sb.String())
}

func buildSections(in []spec.Section, sectionNumbers map[string]struct{}) []Section {
	ret := make([]Section, len(in))
	for i, s := range in {
		ret[i].Section = in[i]
		ret[i].Level = strings.Count(s.Number, ".") + 2
		ret[i].BodyHTML = formatBody(in[i].Body, sectionNumbers)
	}
	return ret
}

func checkSectionLinks(d spec.Document, sectionNumbers, tocNumbers map[string]struct{}) {

	for _, t := range d.TOC {
		if _, ok := sectionNumbers[t.Number]; !ok {
			fmt.Fprintf(os.Stderr, "section %q exists in TOC, but not in sections\n", t.Number)
		}
	}
	for _, s := range d.Sections {
		if _, ok := tocNumbers[s.Number]; !ok {
			fmt.Fprintf(os.Stderr, "section %q exists in sections, but not in TOC\n", s.Number)
		}
		for _, tt := range s.Body {
			if tt.SectionRef != "" {
				if _, ok := sectionNumbers[tt.SectionRef]; !ok {
					fmt.Fprintf(os.Stderr, "non-existing section %q is referenced from section %q\n",
						tt.SectionRef, s.Number)
				}
			}
		}
	}
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
	sectionNumbers := make(map[string]struct{})
	for _, s := range doc.Sections {
		sectionNumbers[s.Number] = struct{}{}
	}
	tocNumbers := make(map[string]struct{})
	for _, t := range doc.TOC {
		tocNumbers[t.Number] = struct{}{}
	}
	checkSectionLinks(doc, sectionNumbers, tocNumbers)
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
		TOCTree:  buildTOCTree(doc.TOC, sectionNumbers),
		Sections: buildSections(doc.Sections, sectionNumbers),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
}
