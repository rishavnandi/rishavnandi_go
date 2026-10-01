package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/url"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"
)

//go:embed templates/*.gohtml
var templatesFS embed.FS

// base defines "layout" (the full document) which renders {{template "content" .}}.
// Each page template contributes its own "content" and "results", so pages are
// parsed into a fresh clone of base rather than into one shared namespace.
var base = template.Must(
	template.New("layout").Funcs(template.FuncMap{
		"iconAt":     iconAt,
		"techIcon":   techIconLookup,
		"formatDate": formatDate,
		"socials":    func() []socialLink { return socials },
	}).ParseFS(templatesFS, "templates/layout.gohtml", "templates/partials.gohtml"))

func pageTemplate(tplFile string) *template.Template {
	clone, err := base.Clone()
	if err != nil {
		panic(err)
	}
	return template.Must(clone.ParseFS(templatesFS, "templates/"+tplFile))
}

type pageData struct {
	Title, Canonical, Description, OGType, OGImage, OGImageAlt string
	SiteName, SiteURL, SocialImage, SearchLabel                string
	Content                                                    template.HTML
	Experiences                                                []experience
	Projects                                                   []project
	Posts                                                      []post
	Post                                                       *post
	Repos                                                      []repo
	Year                                                       int

	// List pages (/projects, /posts)
	Path, Query      string
	Page, TotalPages int
	Total            int // matching items, before paging
	PrevURL, NextURL string
}

func newPage(title, description, canonical, ogType, ogImage string) pageData {
	return pageData{
		SiteName: siteName, SiteURL: siteURL,
		Title: title, Description: description,
		Canonical: canonical, OGType: ogType, OGImage: ogImage, OGImageAlt: siteName,
		Experiences: experiences, Projects: featuredProjects, Posts: posts,
		Year: time.Now().Year(),
	}
}

// pageQuery links to page n of a list, preserving the active search.
func pageQuery(path, q string, n int) string {
	v := url.Values{"page": []string{strconv.Itoa(n)}}
	if q != "" {
		v.Set("q", q)
	}
	return path + "?" + v.Encode()
}

// build prerenders the routes that never vary per request. /projects and /posts
// stay dynamic because htmx re-requests them with ?q=&page=.
func build() map[string][]byte {
	loadPosts()
	out := map[string][]byte{}

	add := func(path, tpl string, d pageData) {
		var buf bytes.Buffer
		if err := pageTemplate(tpl).Execute(&buf, d); err != nil {
			panic(fmt.Sprintf("prerender %s: %v", path, err))
		}
		out[path] = buf.Bytes()
	}

	// Non-HTML output: rendered on its own, no layout wrapper.
	addRaw := func(path, tpl string, d pageData) {
		// text/template: these are XML and plain text, not HTML.
		t := texttemplate.Must(texttemplate.New(tpl).Funcs(texttemplate.FuncMap{
			"trimSpace": strings.TrimSpace,
		}).ParseFS(templatesFS, "templates/"+tpl))
		var buf bytes.Buffer
		if err := t.ExecuteTemplate(&buf, tpl, d); err != nil {
			panic(fmt.Sprintf("prerender %s: %v", path, err))
		}
		out[path] = []byte(strings.TrimSpace(buf.String()) + "\n")
	}

	add("/", "home.gohtml", newPage(
		siteTitle, desc, siteURL+"/", "profile", socialImg))

	for i := range posts {
		p := &posts[i]
		d := newPage(p.Title+" | "+siteName, p.Description,
			siteURL+"/post/"+p.Slug, "article", socialImg)
		d.Content, d.Post = p.HTML, p
		add("/post/"+p.Slug, "post.gohtml", d)
	}

	add("/404", "404.gohtml", newPage(
		"Not Found | "+siteName, "Page not found.",
		siteURL+"/", "website", socialImg))

	// Generated rather than checked in, so a new post cannot be forgotten here.
	addRaw("/sitemap.xml", "sitemap.gohtml", newPage("", "", "", "", ""))
	addRaw("/robots.txt", "robots.gohtml", newPage("", "", "", "", ""))

	return out
}

// paginate clamps the requested page and fills the prev/next links.
func (d *pageData) paginate(total, size int) {
	d.Total = total
	d.TotalPages = totalPages(total, size)
	d.Page = clampPage(d.Page, d.TotalPages)
	if d.Page > 1 {
		d.PrevURL = pageQuery(d.Path, d.Query, d.Page-1)
	}
	if d.Page < d.TotalPages {
		d.NextURL = pageQuery(d.Path, d.Query, d.Page+1)
	}
}

func totalPages(n, size int) int {
	if n < 1 {
		return 1
	}
	return (n + size - 1) / size
}

func clampPage(p, total int) int {
	if p < 1 {
		return 1
	}
	if p > total {
		return total
	}
	return p
}
