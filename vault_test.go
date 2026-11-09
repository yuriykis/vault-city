package main

import (
	"maps"
	"strings"
	"testing"
	"testing/fstest"
)

// "Żaba" composed (NFC), as links are typed, and decomposed (NFD), as macOS
// may store the file name.
const (
	zabaNFC = "Żaba"
	zabaNFD = "Żaba"
)

// files is an in-memory vault: path → note text.
type files map[string]string

// load reads an in-memory vault, so tests never touch a real one.
func load(t *testing.T, vault files) Vault {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, text := range vault {
		fsys[name] = &fstest.MapFile{Data: []byte(text)}
	}
	v, err := LoadVault(fsys)
	if err != nil {
		t.Fatalf("LoadVault: %v", err)
	}
	return v
}

// inbound maps every note name to its inbound link count (building height).
func inbound(v Vault) map[string]int {
	m := make(map[string]int, len(v.Notes))
	for _, n := range v.Notes {
		m[n.Name] = n.Inbound
	}
	return m
}

// TestLoadVaultLinkForms checks which link forms in note A count as a link
// to note B, the way Obsidian's backlinks panel counts them.
func TestLoadVaultLinkForms(t *testing.T) {
	tests := []struct {
		name    string
		text    string // content of Notes/A.md
		counted bool
	}{
		{"plain", "[[B]]", true},
		{"alias", "[[B|bee]]", true},
		{"heading", "[[B#Section]]", true},
		{"heading and alias", "[[B#Section|bee]]", true},
		{"block reference", "[[B#^abc123]]", true},
		{"md suffix", "[[B.md]]", true},
		{"Notes folder", "[[Notes/B]]", true},
		{"embed", "![[B]]", true},
		{"other case", "[[b]]", true},
		{"frontmatter value", "---\nrelated: \"[[B]]\"\n---\n", true},
		{"frontmatter list", "---\nrelated:\n  - \"[[A]]\"\n  - \"[[B]]\"\n---\n", true},
		{"after inline code", "`code` [[B]]", true},
		{"after closed fence", "```\ncode\n```\n[[B]]", true},
		{"CRLF frontmatter", "---\r\nrelated: \"[[B]]\"\r\n---\r\n", true},
		{"CRLF fence", "```\r\n[[B]]\r\n```\r\n", false},
		{"other folder", "[[Projects/B]]", false},
		{"backtick fence", "```\n[[B]]\n```", false},
		{"backtick fence with language", "```go\n[[B]]\n```", false},
		{"tilde fence", "~~~\n[[B]]\n~~~", false},
		{"inline code", "`[[B]]`", false},
		{"other note", "[[Missing]]", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := load(t, files{"Notes/A.md": tt.text, "Notes/B.md": ""})
			want := map[string]int{"A": 0, "B": 0}
			if tt.counted {
				want["B"] = 1
			}
			if got := inbound(v); !maps.Equal(got, want) {
				t.Errorf("Notes/A.md = %q: inbound = %v, want %v", tt.text, got, want)
			}
		})
	}
}

// TestLoadVaultInbound is the regression contract for building heights:
// inbound = Obsidian backlinks from notes in Notes/, frontmatter included
// (decisions R3a, R3b).
func TestLoadVaultInbound(t *testing.T) {
	tests := []struct {
		name  string
		vault files
		want  map[string]int
	}{
		{
			name: "each linking note counts once",
			vault: files{
				"Notes/A.md": "---\nrelated: \"[[B]]\"\n---\n[[B]] [[B|again]] ![[B]]",
				"Notes/B.md": "",
			},
			want: map[string]int{"A": 0, "B": 1},
		},
		{
			name: "every linking note counts",
			vault: files{
				"Notes/A.md": "[[B]]",
				"Notes/B.md": "[[C]]",
				"Notes/C.md": "[[B]]",
			},
			want: map[string]int{"A": 0, "B": 2, "C": 1},
		},
		{
			name:  "self-link does not count",
			vault: files{"Notes/A.md": "[[A]] ![[A]]"},
			want:  map[string]int{"A": 0},
		},
		{
			name: "links from outside Notes do not count",
			vault: files{
				"Notes/B.md":             "",
				"Projects/Plan.md":       "[[B]]",
				"Daily/2026-11-05.md":    "[[Notes/B]]",
				"Home.md":                "[[B]]",
				".obsidian/workspace.md": "[[B]]",
			},
			want: map[string]int{"B": 0},
		},
		{
			name: "only visible Markdown files directly in Notes are notes",
			vault: files{
				"Notes/B.md":         "",
				"Notes/sub/C.md":     "",
				"Notes/.hidden.md":   "",
				"Notes/picture.png":  "",
				"Notes/Board.canvas": "",
				"Notes/Draft.md.bak": "",
			},
			want: map[string]int{"B": 0},
		},
		{
			name: "decomposed file name is found by a composed link",
			vault: files{
				"Notes/A.md":               "[[" + zabaNFC + "]]",
				"Notes/" + zabaNFD + ".md": "",
			},
			want: map[string]int{"A": 0, zabaNFC: 1},
		},
		{
			name: "composed file name is found by a decomposed link",
			vault: files{
				"Notes/A.md":               "[[" + zabaNFD + "]]",
				"Notes/" + zabaNFC + ".md": "",
			},
			want: map[string]int{"A": 0, zabaNFC: 1},
		},
		{
			name:  "empty vault",
			vault: files{"Notes/.keep": ""},
			want:  map[string]int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inbound(load(t, tt.vault)); !maps.Equal(got, tt.want) {
				t.Errorf("inbound = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLoadVaultProperties checks the district (topic) and color (category)
// read from the frontmatter of Notes/A.md.
func TestLoadVaultProperties(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		wantTopic    string
		wantCategory string
	}{
		{
			name: "no frontmatter",
			text: "# A\n\ntopic: \"[[Cooking]]\"\n",
		},
		{
			name:         "links",
			text:         "---\ntopic: \"[[Cooking]]\"\ncategory: \"[[Knowledge]]\"\n---\n",
			wantTopic:    "Cooking",
			wantCategory: "Knowledge",
		},
		{
			name:         "links with folder and alias",
			text:         "---\ntopic: \"[[Notes/Cooking|Gotowanie]]\"\ncategory: \"[[Categories/Knowledge|Knowledge]]\"\n---\n",
			wantTopic:    "Cooking",
			wantCategory: "Knowledge",
		},
		{
			name:      "plain text",
			text:      "---\ntopic: Cooking\n---\n",
			wantTopic: "Cooking",
		},
		{
			name:      "single quotes",
			text:      "---\ntopic: '[[Cooking]]'\n---\n",
			wantTopic: "Cooking",
		},
		{
			name:      "block list takes the first item",
			text:      "---\ntopic:\n  - \"[[Cooking]]\"\n  - \"[[Baking]]\"\n---\n",
			wantTopic: "Cooking",
		},
		{
			name:         "empty topic followed by another key",
			text:         "---\ntopic:\ncategory: \"[[Knowledge]]\"\n---\n",
			wantCategory: "Knowledge",
		},
		{
			name:      "decomposed letters become composed",
			text:      "---\ntopic: \"[[" + zabaNFD + "]]\"\n---\n",
			wantTopic: zabaNFC,
		},
		{
			name:         "CRLF line endings",
			text:         "---\r\ntopic: \"[[Cooking]]\"\r\ncategory: \"[[Knowledge]]\"\r\n---\r\nText\r\n",
			wantTopic:    "Cooking",
			wantCategory: "Knowledge",
		},
		{
			name:      "inline list takes the first item",
			text:      "---\ntopic: [\"[[Cooking]]\", \"[[Baking]]\"]\n---\n",
			wantTopic: "Cooking",
		},
		{
			name: "null",
			text: "---\ntopic: null\ncategory: ~\n---\n",
		},
		{
			name: "empty values",
			text: "---\ntopic: []\ncategory: \"\"\n---\n",
		},
		{
			name: "only similar keys",
			text: "---\ntopics: \"[[Cooking]]\"\nmy category: \"[[Knowledge]]\"\n---\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := load(t, files{"Notes/A.md": tt.text})
			n := v.Notes[0]
			if n.Topic != tt.wantTopic || n.Category != tt.wantCategory {
				t.Errorf("topic, category = %q, %q; want %q, %q",
					n.Topic, n.Category, tt.wantTopic, tt.wantCategory)
			}
			if len(v.Warnings) > 0 {
				t.Errorf("unexpected warnings: %q", v.Warnings)
			}
		})
	}
}

// TestLoadVaultWarnings checks that frontmatter which can't be read gives
// a warning naming the note and empty values, not a district made of raw
// text.
func TestLoadVaultWarnings(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		wantCategory string
		wantWarnings int
	}{
		{
			name:         "map under topic",
			text:         "---\ntopic:\n  name: Cooking\ncategory: \"[[Knowledge]]\"\n---\n",
			wantCategory: "Knowledge",
			wantWarnings: 1,
		},
		{
			name:         "unquoted link is a nested list in YAML",
			text:         "---\ntopic: [[Cooking]]\ncategory: \"[[Knowledge]]\"\n---\n",
			wantCategory: "Knowledge",
			wantWarnings: 1,
		},
		{
			name:         "both properties unreadable",
			text:         "---\ntopic:\n  - [[Cooking]]\ncategory:\n  color: red\n---\n",
			wantWarnings: 2,
		},
		{
			name:         "invalid YAML",
			text:         "---\ntopic: \"[[Cooking]]\"\ncategory: [unclosed\n---\n",
			wantWarnings: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := load(t, files{"Notes/A.md": tt.text, "Notes/B.md": "[[A]]"})
			if len(v.Notes) != 2 || v.Notes[0].Inbound != 1 {
				t.Errorf("notes = %+v, want A with 1 inbound link and B", v.Notes)
			}
			n := v.Notes[0]
			if n.Topic != "" || n.Category != tt.wantCategory {
				t.Errorf("topic, category = %q, %q; want %q, %q",
					n.Topic, n.Category, "", tt.wantCategory)
			}
			if len(v.Warnings) != tt.wantWarnings {
				t.Fatalf("warnings = %q, want %d", v.Warnings, tt.wantWarnings)
			}
			for _, w := range v.Warnings {
				if !strings.HasPrefix(w, "Notes/A.md: ") {
					t.Errorf("warning %q does not start with the note's path", w)
				}
			}
		})
	}
}

// TestLoadVaultLinkTotal checks the wikilink total from the terminal
// summary: every link outside code, resolved or not, repeats included.
func TestLoadVaultLinkTotal(t *testing.T) {
	v := load(t, files{
		"Notes/A.md":       "---\nrelated: \"[[B]]\"\n---\n[[B]] [[Missing]] ![[B]] `[[C]]`\n```\n[[D]]\n```\n",
		"Notes/B.md":       "[[A]]",
		"Projects/Plan.md": "[[A]]",
	})
	if v.Links != 5 {
		t.Errorf("Links = %d, want 5", v.Links)
	}
}

func TestLoadVaultWithoutNotes(t *testing.T) {
	fsys := fstest.MapFS{"Home.md": &fstest.MapFile{Data: []byte("[[B]]")}}
	if _, err := LoadVault(fsys); err == nil {
		t.Error("LoadVault without a Notes folder: got nil error")
	}
}
