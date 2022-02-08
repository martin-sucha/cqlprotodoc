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

func link(sb *strings.Builder, href, text string) {
	sb.WriteString(`<a href="`)
	sb.WriteString(template.HTMLEscapeString(href))
	sb.WriteString(`">`)
	sb.WriteString(template.HTMLEscapeString(text))
	sb.WriteString(`</a>`)
}

func formatBody(text []spec.Text) template.HTML {
	var sb strings.Builder
	for _, t := range text {
		switch {
		case t.SectionRef != "":
			link(&sb, "#s"+t.SectionRef, t.Text)
		case t.Href != "":
			link(&sb, t.Href, t.Text)
		default:
			sb.WriteString(template.HTMLEscapeString(t.Text))
		}
	}
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
