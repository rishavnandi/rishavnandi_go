# rishavnandi.com — Go + htmx

Rewrite of [rishavnandi.com](https://www.rishavnandi.com) (formerly SvelteKit +
Tailwind) as a single Go binary. Same routes, same content, same look.

```bash
go run .            # http://localhost:8080
go test ./...
```

## Why it's fast

Every route except the two search pages is rendered **once at startup** into a
`map[string][]byte`. Serving is a map lookup and a `Write` — no template
execution, no markdown parsing, no allocation churn on the hot path.

| | ns/op | allocs/op |
|---|---|---|
| `/` | ~10,000 | 23 |
| `/post/homelab` | ~5,000 | 28 |
| htmx fragment | ~59,000 | 523 |

`/` and post pages ship **zero JavaScript**. htmx (37KB) loads only on
`/projects` and `/posts`, and only there does it do anything.

Payload per visit, and throughput against the previous SvelteKit build served
behind an equivalent static server (50 concurrent, keep-alive, M1 Pro):

| | old | new |
|---|---|---|
| `/` homepage | 145 KB / 24.6 KB gzip | 66 KB / 13.1 KB gzip |
| `/posts` | 305 KB / 94.2 KB gzip | 69 KB / 22.1 KB gzip |
| JS files on `/` | 1 (41 KB CSS) | 0 |
| rps, `/` | 28,441 | 49,602 (1.74x) |
| RSS at rest | — | ~28 MB |

`/posts` is the one page slower than a static file (0.44x), because it renders
per request so htmx can query it. At ~16k rps that is not a practical concern.

## Layout

```
main.go       routing, static files, GitHub fetching, list filtering
render.go     template setup, prerender pass, pagination
content.go    markdown posts: frontmatter parse + goldmark render
site.go       hand-written content: experience, projects, socials
icons.go      inlined SVGs (Lucide strokes + brand fills)
templates/    layout.gohtml holds the document; one file per route
static/       site.css, htmx.min.js, images
content/      the posts, verbatim from the Svelte version
```

Templates use one `pageData` struct rather than a type per page, and each route
parses into a clone of `base` so pages can each define `"content"`.

## htmx 4 notes

htmx 4 dropped implicit attribute inheritance, so `hx-target` and `hx-swap` are
declared on the elements that actually swap. Search hits `/projects?q=...` and
swaps `outerHTML` of `#results`, which is why the fragment includes the wrapper
itself.

Both list routes also work with JavaScript disabled: a plain `GET` with `?q=`
returns the full page. The count lives inside `#results` so it updates with the
results.

## GitHub repos

`/projects` fetches from the GitHub API on demand and caches the result for five
minutes, so a new public repo shows up on its own without a redeploy.

Fetched lazily rather than from a background ticker: a ticker needs a
long-lived process and silently returns nothing on serverless hosts, where the
process is frozen between requests. Lazy fetch works everywhere. The response
carries `s-maxage=300` so a CDN absorbs the calls, which keeps a serverless cold
start from burning the 60 unauthenticated GitHub requests/hour. On failure the
last good list is kept, so a GitHub outage never blanks the page.

Unauthenticated requests are capped at 60/hour per IP. `GITHUB_TOKEN` is not
wired up; add it if the TTL ever needs to drop.

The homepage "Projects" section is a separate, hand-curated list in `site.go` and
does **not** auto-populate. That matches the original: `/projects` is the
automatic one.

## Deliberate omissions

Dropped from the original, per the migration decision:

- **shiki** syntax highlighting — code blocks are styled `<pre><code>`. A
  JavaScript highlighter contradicts the goal.
- **per-post OG images** (`/api/og/{slug}`) — `og:image` points at the static
  social image instead.

`llms.txt`, `robots.txt` and `sitemap.xml` are generated at startup from the
loaded posts, so adding a post can't leave them stale.

## Deploying

Set `PORT` and run the binary. `vercel.json` sets the `go` framework preset;
Vercel's Go runtime is Beta. Fly.io, Render, Railway, Coolify or any VPS run the
same binary unchanged.

## Content

Posts live in `content/*.md` with YAML frontmatter (`published: false` hides a
post). Add a file, restart. Note the original files have CRLF endings; the parser
normalises them.

## Credits

- [htmx](https://htmx.org) 4.0.0, bundled in `static/` — BSD 3-Clause
- [goldmark](https://github.com/yuin/goldmark) for markdown — MIT
- Icon paths from [Lucide](https://lucide.dev) (ISC) and brand marks from the
  previous Svelte components