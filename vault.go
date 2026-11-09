package main

import (
	"cmp"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
	"golang.org/x/text/unicode/norm"
)

// notesDir is the only vault folder the city is built from.
const notesDir = "Notes"

// Note is one Markdown file from Notes/: a future building.
type Note struct {
	Name     string // file name without ".md"
	Topic    string // district; empty for the suburbs
	Category string // building color
	Inbound  int    // how many other notes in Notes/ link here
}

// Vault is everything the game knows about the vault.
type Vault struct {
	Notes    []Note
	Links    int      // every [[wikilink]] found in Notes/, resolved or not
	Warnings []string // frontmatter that could not be read, one per problem
}

var (
	wikilink   = regexp.MustCompile(`\[\[([^\]\n]+)\]\]`)
	inlineCode = regexp.MustCompile("`[^`\n]*`")
)

// LoadVault reads Notes/*.md from vault. It takes an fs.FS, which has no
// write methods, so the game cannot change the vault through it.
func LoadVault(vault fs.FS) (Vault, error) {
	entries, err := fs.ReadDir(vault, notesDir)
	if err != nil {
		return Vault{}, err
	}

	var v Vault
	var outgoing [][]string // raw link texts of v.Notes[i]
	for _, e := range entries {
		name, isMarkdown := strings.CutSuffix(e.Name(), ".md")
		if e.IsDir() || !isMarkdown || strings.HasPrefix(name, ".") {
			continue
		}
		file := path.Join(notesDir, e.Name())
		data, err := fs.ReadFile(vault, file)
		if err != nil {
			return Vault{}, err
		}
		text := string(data)
		props, err := frontmatter(text)
		if err != nil {
			v.Warnings = append(v.Warnings, fmt.Sprintf(
				"%s: frontmatter is not valid YAML, topic and category ignored: %v", file, err))
		}
		topic, topicErr := propertyName(props, "topic")
		category, categoryErr := propertyName(props, "category")
		for _, err := range []error{topicErr, categoryErr} {
			if err != nil {
				v.Warnings = append(v.Warnings, fmt.Sprintf("%s: %v", file, err))
			}
		}
		v.Notes = append(v.Notes, Note{
			// macOS may store file names decomposed (NFD), while links in note
			// text are composed (NFC); normalize so "ż" matches "ż".
			Name:     norm.NFC.String(name),
			Topic:    topic,
			Category: category,
		})
		links := linkTexts(text)
		outgoing = append(outgoing, links)
		v.Links += len(links)
	}

	// Like Obsidian's backlinks: each linking note counts once, self-links don't.
	index := make(map[string]int, len(v.Notes))
	for i, n := range v.Notes {
		index[noteKey(n.Name)] = i
	}
	for src, links := range outgoing {
		seen := make(map[int]bool)
		for _, link := range links {
			dir, name := parseLink(link)
			if dir != "" && dir != notesDir {
				continue // e.g. [[Projects/Plan]] points outside Notes/
			}
			dst, ok := index[noteKey(name)]
			if !ok || dst == src || seen[dst] {
				continue
			}
			seen[dst] = true
			v.Notes[dst].Inbound++
		}
	}
	return v, nil
}

// frontmatter parses the YAML block between the leading "---" lines into
// its properties; a note without one has none. LF and CRLF both work.
func frontmatter(text string) (map[string]yaml.Node, error) {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		rest, ok = strings.CutPrefix(text, "---\r\n")
	}
	if !ok {
		return nil, nil
	}
	front, _, ok := strings.Cut(rest, "\n---")
	if !ok {
		return nil, nil
	}
	var props map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(front), &props); err != nil {
		return nil, err
	}
	return props, nil
}

// propertyName returns the note name a frontmatter property points to, so
// `topic: "[[Cooking]]"` gives "Cooking". It reads text or the first item of
// a list; "" means the property is unset, null or an empty list. Any other
// shape is an error, so a strange note never becomes a strange district.
func propertyName(props map[string]yaml.Node, key string) (string, error) {
	node, ok := props[key]
	if !ok {
		return "", nil
	}
	if node.Kind == yaml.SequenceNode {
		if len(node.Content) == 0 {
			return "", nil
		}
		node = *node.Content[0]
	}
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s is not text or a list of text, ignored "+
			"(a link must be quoted: %s: \"[[Note]]\")", key, key)
	}
	if node.ShortTag() == "!!null" {
		return "", nil
	}
	value := node.Value
	if inner, isLink := strings.CutPrefix(value, "[["); isLink {
		_, value = parseLink(strings.TrimSuffix(inner, "]]"))
	}
	return norm.NFC.String(value), nil
}

// linkTexts returns the text inside every [[wikilink]] and ![[embed]],
// skipping code blocks and inline code, where Obsidian ignores links.
func linkTexts(text string) []string {
	var out []string
	inFence := false
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		for _, m := range wikilink.FindAllStringSubmatch(inlineCode.ReplaceAllString(line, ""), -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// parseLink splits the text of [[folder/Note#Heading|alias]] into
// ("folder", "Note").
func parseLink(link string) (dir, name string) {
	link, _, _ = strings.Cut(link, "|")
	link, _, _ = strings.Cut(link, "#")
	link = strings.TrimSuffix(strings.TrimSpace(link), ".md")
	dir, name = path.Split(link)
	return strings.TrimSuffix(dir, "/"), name
}

// noteKey is how a link finds its note: case-insensitive, Unicode-normalized.
func noteKey(name string) string {
	return strings.ToLower(norm.NFC.String(name))
}

// printSummary writes what the city will be built from.
func printSummary(w io.Writer, v Vault) {
	topics := countBy(v.Notes, func(n Note) string { return n.Topic })
	suburbs := topics[""]
	delete(topics, "")
	inbound := make(map[string]int, len(v.Notes))
	for _, n := range v.Notes {
		inbound[n.Name] = n.Inbound
	}

	fmt.Fprintf(w, "Notes: %d, wikilinks: %d, districts: %d, suburbs (no topic): %d\n",
		len(v.Notes), v.Links, len(topics), suburbs)
	printTop(w, "Largest districts", topics, 10)
	printTop(w, "Categories (colors)", countBy(v.Notes, func(n Note) string { return n.Category }), 15)
	printTop(w, "Most linked (tallest buildings)", inbound, 10)
}

func countBy(notes []Note, field func(Note) string) map[string]int {
	counts := make(map[string]int)
	for _, n := range notes {
		counts[field(n)]++
	}
	return counts
}

// printTop prints the limit largest counts, ties in name order.
func printTop(w io.Writer, title string, counts map[string]int, limit int) {
	names := slices.Collect(maps.Keys(counts))
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(cmp.Compare(counts[b], counts[a]), strings.Compare(a, b))
	})
	fmt.Fprintf(w, "\n%s:\n", title)
	for _, name := range names[:min(limit, len(names))] {
		fmt.Fprintf(w, "%6d  %s\n", counts[name], cmp.Or(name, "(none)"))
	}
}
