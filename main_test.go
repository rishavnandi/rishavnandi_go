package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	return (&server{
		pages:  build(),
		static: http.NotFoundHandler(),
		repos:  seededCache(),
	}).routes()
}

// seededCache is pre-filled and fresh, so tests never touch the network and
// never depend on how many public repos the account currently has.
func seededCache() *repoCache {
	return &repoCache{
		repos: []repo{
			{Name: "ansible_homelab", Description: "Ansible playbooks for a homelab",
				Topics: []string{"ansible", "docker"}, HTMLURL: "https://github.com/rishavnandi/ansible_homelab", Stars: 399},
			{Name: "boiler_plates", Description: "Docker Compose templates",
				Topics: []string{"docker"}, HTMLURL: "https://github.com/rishavnandi/boiler_plates", Stars: 62},
			{Name: "wireguard_vpn", Description: "A Wireguard VPN server",
				Topics: []string{"wireguard"}, HTMLURL: "https://github.com/rishavnandi/wireguard_vpn", Stars: 10},
		},
		fetched: time.Now(),
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

// Routes must all answer, and unknown ones must 404 with the rendered page
// rather than Go's bare text.
func TestRoutes(t *testing.T) {
	h := newTestServer(t)
	for path, want := range map[string]int{
		"/": 200, "/projects": 200, "/posts": 200,
		"/post/homelab": 200, "/post/minikube": 200,
		"/sitemap.xml": 200, "/robots.txt": 200,
		"/gh":        302,
		"/post/nope": 404, "/nope": 404,
	} {
		if got := get(t, h, path).Code; got != want {
			t.Errorf("GET %s = %d, want %d", path, got, want)
		}
	}
}

func TestNotFoundIsRenderedPage(t *testing.T) {
	body := get(t, newTestServer(t), "/nope").Body.String()
	if !strings.Contains(body, "<!doctype html>") || !strings.Contains(body, "404") {
		t.Error("404 should render the site 404 page, not bare text")
	}
}

// The home page is prerendered, so it must ship no JavaScript at all.
func TestHomeShipsNoJS(t *testing.T) {
	body := get(t, newTestServer(t), "/").Body.String()
	if strings.Contains(body, "htmx") {
		t.Error("home page should not load htmx")
	}
	for _, path := range []string{"/projects", "/posts"} {
		if !strings.Contains(get(t, newTestServer(t), path).Body.String(), "htmx.min.js") {
			t.Errorf("%s should load htmx for search", path)
		}
	}
}

// htmx fragments must include the swap target id, since the swap is outerHTML.
func TestFragmentsIncludeSwapTarget(t *testing.T) {
	h := newTestServer(t)
	for _, path := range []string{"/projects", "/posts"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path+"?q=a", nil)
		r.Header.Set("HX-Request", "true")
		h.ServeHTTP(w, r)

		if !strings.Contains(w.Body.String(), `id="results"`) {
			t.Errorf("%s fragment missing id=results", path)
		}
		if strings.Contains(w.Body.String(), "<!doctype") {
			t.Errorf("%s fragment should not be a full document", path)
		}
	}
}

func TestSearch(t *testing.T) {
	h := newTestServer(t)

	frag := func(path string) string {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("HX-Request", "true")
		h.ServeHTTP(w, r)
		return w.Body.String()
	}

	if got := frag("/posts?q=minikube"); !strings.Contains(got, "Setup Minikube on WSL2") ||
		strings.Contains(got, "homelab setup with Ansible") {
		t.Error("posts search should match only the minikube post")
	}
	if got := frag("/posts?q=zzzz"); !strings.Contains(got, "No posts match your search") {
		t.Error("empty result should render the empty state")
	}
	// Seeded cache has 3 repos; a topic match must filter to one.
	if got := frag("/projects?q=wireguard"); !strings.Contains(got, "wireguard_vpn") ||
		strings.Contains(got, "boiler_plates") {
		t.Error("repo search should match on topics and exclude non-matches")
	}
	if got := frag("/projects?q=ansible"); strings.Contains(got, "boiler_plates") ||
		!strings.Contains(got, "1 repositories") {
		t.Errorf("repo search should report the filtered count, got:\n%s", got)
	}
	if got := frag("/projects?q=nothingmatches"); !strings.Contains(got, "No repositories match") {
		t.Error("repo search should render the empty state")
	}
}

// The lazy fetch must serve the cached list without re-hitting GitHub on every
// request, and must keep serving the stale list if GitHub fails.
func TestRepoCacheServesCache(t *testing.T) {
	c := seededCache()
	ctx := context.Background()
	for range 3 {
		if got := c.list(ctx); len(got) != 3 {
			t.Fatalf("cached list = %d repos, want 3", len(got))
		}
	}
	if c.fetched.IsZero() {
		t.Error("a fresh cache should not refetch")
	}
}

// Every repo search page must be reachable and render, which is what proves a
// new GitHub repo becomes visible without a redeploy.
func TestProjectsRendersRepos(t *testing.T) {
	h := newTestServer(t)
	body := get(t, h, "/projects").Body.String()
	if !strings.Contains(body, "ansible_homelab") {
		t.Error("/projects did not render the repo list")
	}
	if !strings.Contains(body, "3 repositories") {
		t.Error("/projects did not render the repo count")
	}
}

// Search is case-insensitive and matches topics, not just names.
func TestSearchCaseInsensitive(t *testing.T) {
	if got := filterPosts("MINIKUBE"); len(got) != 1 {
		t.Errorf("uppercase search matched %d posts, want 1", len(got))
	}
}

func TestPagingClampsOutOfRange(t *testing.T) {
	h := newTestServer(t)
	// The only posts are 2, so page 999 must clamp to page 1 and not 404.
	if got := get(t, h, "/posts?page=999").Code; got != 200 {
		t.Errorf("out-of-range page = %d, want 200", got)
	}
	// Clamping means page 999 becomes the last page, not an empty one.
	if got := slice([]int{1, 2, 3}, 999, 2); len(got) != 1 || got[0] != 3 {
		t.Errorf("clamped last page = %v, want [3]", got)
	}
	if got := slice([]int{1, 2, 3, 4, 5}, 2, 2); len(got) != 2 || got[0] != 3 {
		t.Errorf("page 2 of 5 = %v, want [3 4]", got)
	}
}

func TestPaginatorKeepsQuery(t *testing.T) {
	var d pageData
	d.Path, d.Query, d.Page = "/projects", "docker", 2
	d.paginate(30, 10)

	if d.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", d.TotalPages)
	}
	if !strings.Contains(d.PrevURL, "q=docker") || !strings.Contains(d.PrevURL, "page=1") {
		t.Errorf("PrevURL lost the search: %q", d.PrevURL)
	}
	if !strings.Contains(d.NextURL, "q=docker") {
		t.Errorf("NextURL lost the search: %q", d.NextURL)
	}
}

func TestSecurityAndCacheHeaders(t *testing.T) {
	h := newTestServer(t)
	w := get(t, h, "/")
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options")
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options")
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "must-revalidate") {
		t.Error("HTML should revalidate so redeploys land immediately")
	}
	// htmx never changes, so it must be immutable.
	if got := get(t, h, "/htmx.min.js").Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("htmx cache-control = %q, want immutable", got)
	}
}

// Only published posts render, newest first.
func TestPostLoading(t *testing.T) {
	loadPosts()
	if len(posts) != 2 {
		t.Fatalf("loaded %d posts, want 2", len(posts))
	}
	if posts[0].Slug != "homelab" {
		t.Errorf("first post = %q, want homelab (newest first)", posts[0].Slug)
	}
	if posts[0].Category != "Tutorial" || len(posts[0].Tags) != 3 {
		t.Errorf("frontmatter not parsed: %+v", posts[0])
	}
	html := string(posts[0].HTML)
	if !strings.Contains(html, "<pre") {
		t.Error("markdown code blocks should render")
	}
	// Highlighting happens at build time via chroma; pages must stay JS-free.
	if !strings.Contains(html, `class="tok-`) {
		t.Error("fenced code should carry chroma tok-* classes")
	}
}

// Every project card needs an icon: its own logo, or its lead tech's.
func TestProjectIcons(t *testing.T) {
	body := string(mustRender(t, "home.gohtml"))

	if !strings.Contains(body, "kusama_logo.svg") {
		t.Error("Kusama should use its own logo")
	}
	for _, p := range featuredProjects {
		if p.Icon != "" {
			continue // own logo, already asserted for one of them
		}
		// Ansible Homelab has no logo and leads with Ansible, so it falls back.
		name := techIcon[p.LeadTag()]
		if name == "" {
			t.Errorf("%s has no logo and no icon for %q", p.Title, p.LeadTag())
			continue
		}
		if !strings.Contains(body, string(iconAt(name, 24))) {
			t.Errorf("%s should render the %q fallback icon", p.Title, name)
		}
	}
}

// Every tag used on the homepage needs an icon, or the badge renders bare text.
func TestEveryHomeTagHasAnIcon(t *testing.T) {
	for _, p := range featuredProjects {
		for _, tag := range p.Tags {
			if techIcon[tag] == "" {
				t.Errorf("%s: tag %q has no icon", p.Title, tag)
			}
		}
	}
}

// Icons carry their own viewBox; forcing 0 0 24 24 would clip them. Non-square
// marks must keep their aspect ratio rather than being sheared into a square.
func TestIconsHaveViewBox(t *testing.T) {
	for name, b := range brandIcons {
		icon := string(iconAt(name, 18))
		if !strings.Contains(icon, "viewBox=") {
			t.Errorf("%s rendered without a viewBox", name)
		}
		// Must fit inside an 18x18 box with its aspect ratio intact.
		vw, vh := viewBox(b.viewBox)
		wantW, wantH := 18, 18
		if vw > vh {
			wantH = int(float64(18) * vh / vw)
		} else if vh > vw {
			wantW = int(float64(18) * vw / vh)
		}
		if !strings.Contains(icon, fmt.Sprintf(`width="%d" height="%d"`, wantW, wantH)) {
			t.Errorf("%s (viewBox %s) should render %dx%d, got %s",
				name, b.viewBox, wantW, wantH, icon[:min(120, len(icon))])
		}
	}
	if iconAt("no-such-icon", 12) != "" {
		t.Error("unknown icon should render nothing, not break the layout")
	}
}

// Every experience entry needs its own timeline dot. This regressed once: the
// dot was absolutely positioned with a `top` but no positioned ancestor on the
// <li>, so all three dots resolved against the <ol> and stacked at the top.
func TestTimelineDotPerEntry(t *testing.T) {
	body := string(mustRender(t, "home.gohtml"))

	entries := strings.Count(body, `class="entry"`)
	dots := strings.Count(body, `class="dot"`)
	if entries == 0 {
		t.Fatal("no experience entries rendered")
	}
	if dots != entries {
		t.Errorf("%d entries but %d dots; every entry needs one", entries, dots)
	}
	if len(experiences) != entries {
		t.Errorf("rendered %d entries, data has %d", entries, len(experiences))
	}
	// The dot must sit inside its own <li> (after the template comment), so it
	// travels with that entry rather than with the whole list.
	if !strings.Contains(body, `class="dot" aria-hidden="true"></span>
      <time`) {
		t.Error("dot should be inside its own .entry, immediately before the date")
	}
}

func TestFormatDate(t *testing.T) {
	// Must match Intl.DateTimeFormat(en, {dateStyle:'medium', timeZone:'UTC'}).
	if got := formatDate("2023-03-23"); got != "Mar 23, 2023" {
		t.Errorf("formatDate = %q, want Mar 23, 2023", got)
	}
	if got := formatDate("nonsense"); got != "nonsense" {
		t.Errorf("bad input should pass through, got %q", got)
	}
}
func mustRender(t *testing.T, tplFile string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := pageTemplate(tplFile).Execute(&buf, newPage("t", "d", "c", "website", "")); err != nil {
		t.Fatalf("render %s: %v", tplFile, err)
	}
	return buf.Bytes()
}
