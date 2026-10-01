package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed static
var staticFS embed.FS

const pageSize = 10

type server struct {
	pages  map[string][]byte // prerendered HTML, keyed by request path
	static http.Handler
	repos  *repoCache
}

func main() {
	addr := flag.String("addr", env("ADDR", ":"+env("PORT", "8080")), "listen address")
	flag.Parse()

	pages := build()

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}

	s := &server{
		pages:  pages,
		static: http.FileServer(http.FS(sub)),
		repos:  &repoCache{},
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	slog.Info("listening", "addr", *addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	for path, to := range map[string]string{
		"/tw":  "https://twitter.com/rishav__nandi",
		"/x":   "https://twitter.com/rishav__nandi",
		"/ln":  "https://www.linkedin.com/in/rishavnandi/",
		"/gh":  "https://github.com/rishavnandi",
		"/git": "https://github.com/rishavnandi/rishavnandi.com",
	} {
		mux.Handle("GET "+path, http.RedirectHandler(to, http.StatusFound))
	}

	// The list routes stay dynamic: htmx posts ?q=&page= at them, and a plain
	// GET has to work without JavaScript too. Posts are prerendered, so they fall
	// through to the root handler below.
	mux.HandleFunc("GET /projects", s.listPage(projectsTpl, s.repoQuery))
	mux.HandleFunc("GET /posts", s.listPage(postsTpl, postQuery))

	// Root handler, in order: prerendered HTML, then static assets, then the
	// prerendered 404 body.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if body, ok := s.pages[r.URL.Path]; ok {
			w.Header().Set("Content-Type", contentType(r.URL.Path))
			writeBytes(w, http.StatusOK, body)
			return
		}
		// Files are content-stable, so serve them straight from the embedded FS
		// and only fall back to 404 when the path is not a real file.
		if name := strings.TrimPrefix(path.Clean(r.URL.Path), "/"); name != "" {
			if f, err := staticFS.Open("static/" + name); err == nil {
				f.Close()
				s.static.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		writeBytes(w, http.StatusNotFound, s.pages["/404"])
	})

	return withHeaders(mux)
}

// writeBytes sends a complete body with a known length. Setting Content-Length
// matters: without it Go falls back to chunked transfer encoding, which adds
// chunk headers and blocks any sendfile-style path.
func writeBytes(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(status)
	w.Write(b)
}

// contentType for prerendered pages, which skip the FileServer's sniffing.
func contentType(p string) string {
	switch filepath.Ext(p) {
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "text/html; charset=utf-8"
	}
}

// withHeaders sets the security and cache headers shared by every response.
// htmx never changes, so it gets a one-year immutable cache; everything else
// must revalidate so a redeploy is picked up immediately.
func withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		switch {
		case strings.HasSuffix(r.URL.Path, "htmx.min.js"):
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		case isRepoList(r.URL.Path):
			// The repo list is fetched from GitHub, so it is the one route whose
			// staleness is expected. s-maxage lets a CDN hold it for 5 minutes,
			// which is what keeps a serverless cold start from burning the 60
			// unauthenticated GitHub calls/hour. Matches the original's header.
			h.Set("Cache-Control", "public, max-age=0, s-maxage=300, stale-while-revalidate=60")
		default:
			h.Set("Cache-Control", "public, max-age=0, must-revalidate")
		}
		next.ServeHTTP(w, r)
	})
}

func isRepoList(p string) bool { return p == "/projects" }

const (
	projectsTpl = "projects"
	postsTpl    = "posts"
)

// query builds the page data for a list route from ?q= and ?page=.
// It takes the request context so the repo cache can do a lazy fetch.
type query func(context.Context, pageData, string, int) pageData

func (s *server) listPage(tpl string, q query) http.HandlerFunc {
	t := pageTemplate(tpl + ".gohtml")
	return func(w http.ResponseWriter, r *http.Request) {
		d := q(r.Context(), newPage("", "", "", "", socialImg),
			r.URL.Query().Get("q"), atoiDefault(r.URL.Query().Get("page"), 1))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// htmx asks for just the results list; a plain GET gets the whole page
		// and works identically with JavaScript disabled.
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Cache-Control", "no-store")
			render(w, t, "results", d)
			return
		}
		render(w, t, "layout", d)
	}
}

func render(w http.ResponseWriter, t *template.Template, name string, d pageData) {
	if err := t.ExecuteTemplate(w, name, d); err != nil {
		slog.Error("render", "template", name, "err", err)
	}
}

// ---------- list queries ----------

func (s *server) repoQuery(ctx context.Context, d pageData, q string, page int) pageData {
	d.Path, d.Query, d.Page = "/projects", q, page
	d.Title = "Projects | " + siteName
	d.Description = "Open-source AI, platform engineering, DevOps, and self-hosting projects by " + siteName + "."
	d.Canonical, d.OGType, d.OGImage, d.OGImageAlt =
		siteURL+"/projects", "website", socialImg, siteName+" projects"
	d.SearchLabel = "Search Repositories"

	all := s.repos.list(ctx)
	filtered := filterRepos(all, q)
	d.Repos = slice(filtered, d.Page, pageSize)
	d.paginate(len(filtered), pageSize)
	return d
}

func postQuery(_ context.Context, d pageData, q string, page int) pageData {
	d.Path, d.Query, d.Page = "/posts", q, page
	d.Title = "Posts | " + siteName
	d.Description = "Technical writing by " + siteName + " about AI platforms, DevOps, automation, and self-hosted systems."
	d.Canonical, d.OGType, d.OGImage, d.OGImageAlt =
		siteURL+"/posts", "website", socialImg, siteName+" technical writing"
	d.SearchLabel = "Search Posts"

	filtered := filterPosts(q)
	d.Posts = slice(filtered, d.Page, pageSize)
	d.paginate(len(filtered), pageSize)
	return d
}

func filterRepos(all []repo, q string) []repo {
	return filter(all, q, func(r repo) string {
		return r.Name + " " + r.Description + " " + strings.Join(r.Topics, " ")
	})
}

func filterPosts(q string) []post {
	all := posts
	return filter(all, q, func(p post) string {
		return p.Title + " " + p.Description + " " + strings.Join(p.Tags, " ")
	})
}

func filter[T any](items []T, q string, hay func(T) string) []T {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return items
	}
	out := make([]T, 0, len(items))
	for _, it := range items {
		if strings.Contains(strings.ToLower(hay(it)), q) {
			out = append(out, it)
		}
	}
	return out
}

func slice[T any](items []T, page, size int) []T {
	page = clampPage(page, totalPages(len(items), size))
	start := (page - 1) * size
	if start >= len(items) {
		return nil
	}
	return items[start:min(start+size, len(items))]
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ---------- GitHub ----------

// repoCache holds the repo list for repoTTL so a burst of requests costs one
// GitHub call, not one per request.
//
// ponytail: fetch on demand rather than from a background ticker. A ticker only
// works on a long-lived process and silently returns nothing on serverless
// (Vercel/Lambda/Workers freeze the process between requests). Lazy fetch works
// on every host, and one waiter pays the fetch while the rest read the result.
const repoTTL = 5 * time.Minute

type repoCache struct {
	mu      sync.Mutex
	repos   []repo
	fetched time.Time
	loading bool
	ready   chan struct{} // closed once repos is populated
}

// list returns the cached repos, refetching if stale. On fetch failure it
// returns whatever it last had, so a GitHub outage never blanks the page.
func (c *repoCache) list(ctx context.Context) []repo {
	c.mu.Lock()
	if !c.fetched.IsZero() && time.Since(c.fetched) < repoTTL {
		got := c.repos
		c.mu.Unlock()
		return got
	}
	if c.loading {
		// Another request is already fetching; wait for it rather than stampede.
		ready := c.ready
		c.mu.Unlock()
		select {
		case <-ready:
		case <-ctx.Done():
		case <-time.After(10 * time.Second): // never hang a request on GitHub
		}
		c.mu.Lock()
		got := c.repos
		c.mu.Unlock()
		return got
	}
	c.loading = true
	c.ready = make(chan struct{})
	ready := c.ready
	c.mu.Unlock()

	got, err := fetchRepos(ctx)

	c.mu.Lock()
	c.loading = false
	if err == nil {
		slices.SortStableFunc(got, func(a, b repo) int { return b.Stars - a.Stars })
		c.repos, c.fetched = got, time.Now()
		slog.Info("github repos", "count", len(got))
	} else {
		slog.Error("github", "err", err)
		// Back off so a persistent outage does not hammer the API on every
		// request; we keep serving the stale list.
		c.fetched = time.Now()
	}
	stale := c.repos
	c.mu.Unlock()
	close(ready)
	return stale
}

func fetchRepos(ctx context.Context) ([]repo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	var all []repo

	for page := 1; ; page++ {
		u := fmt.Sprintf("https://api.github.com/users/rishavnandi/repos?type=owner&sort=updated&per_page=100&page=%d", page)
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		var batch []struct {
			Name        string   `json:"name"`
			Description *string  `json:"description"`
			Topics      []string `json:"topics"`
			HTMLURL     string   `json:"html_url"`
			Stars       int      `json:"stargazers_count"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&batch)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github: %s", resp.Status)
		}

		for _, b := range batch {
			d := ""
			if b.Description != nil {
				d = *b.Description
			}
			all = append(all, repo{
				Name: b.Name, Description: d, Topics: b.Topics,
				HTMLURL: b.HTMLURL, Stars: b.Stars,
			})
		}
		if len(batch) < 100 {
			return all, nil
		}
	}
}
