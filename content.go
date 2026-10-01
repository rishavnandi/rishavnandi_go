package main

import (
	"bytes"
	"embed"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed content/*.md
var contentFS embed.FS

type post struct {
	Slug, Title, Description, Date, Category, Updated string
	Writing, Published                                bool
	Tags                                              []string
	HTML                                              template.HTML
	body                                              []byte
}

// posts is sorted newest-first and only holds published posts.
var posts []post

func loadPosts() {
	files, err := contentFS.ReadDir("content")
	if err != nil {
		panic(err)
	}
	// Reset: build() calls this, and tests call it again.
	posts = nil
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))

	for _, f := range files {
		raw, err := contentFS.ReadFile("content/" + f.Name())
		if err != nil {
			panic(err)
		}
		p := parseFrontmatter(raw)
		p.Slug = strings.TrimSuffix(f.Name(), ".md")
		if !p.Published {
			continue
		}
		var buf bytes.Buffer
		if err := md.Convert(p.body, &buf); err != nil {
			panic(err)
		}
		p.HTML = template.HTML(buf.Bytes())
		posts = append(posts, p)
	}
	sort.Slice(posts, func(i, j int) bool { return posts[i].Date > posts[j].Date })
}

// frontmatter splits `---\nyaml\n---\n\nbody`, then reads the handful of keys we
// actually use. A full YAML parser is not worth a dependency for six fields.
func parseFrontmatter(raw []byte) post {
	// The original posts have CRLF line endings; normalising keeps the "---"
	// delimiters matching.
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	var p post

	rest := strings.TrimPrefix(s, "---\n")
	head, body, _ := strings.Cut(rest, "\n---\n")
	p.body = []byte(body)

	for line := range strings.SplitSeq(head, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "title":
			p.Title = unquote(val)
		case "description":
			p.Description = unquote(val)
		case "date":
			p.Date = unquote(val)
		case "lastUpdated":
			p.Updated = unquote(val)
		case "category":
			p.Category = unquote(val)
		case "published":
			p.Published, _ = strconv.ParseBool(val)
		case "writing":
			p.Writing, _ = strconv.ParseBool(val)
		case "tags":
			p.Tags = parseTags(val)
		}
	}
	return p
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

func parseTags(s string) []string {
	s = strings.Trim(s, "[]")
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if part = unquote(strings.TrimSpace(part)); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// formatDate matches Intl.DateTimeFormat(en, {dateStyle:'medium', timeZone:'UTC'}):
// "Mar 23, 2023".
func formatDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.UTC().Format("Jan 2, 2006")
}
