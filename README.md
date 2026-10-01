# rishavnandi.com

Source for [rishavnandi.com](https://www.rishavnandi.com). Go and htmx 4,
deployed on Vercel.

```bash
go run .            # http://localhost:8080
go test ./...
```

## How it works

Every route except `/projects` and `/posts` is rendered once at startup into a
`map[string][]byte`, so serving is a map lookup and a write. Those two render per
request so htmx can search them, and both work without JavaScript too.

htmx loads only on those two pages. The homepage and post pages ship no
JavaScript at all, and code highlighting is chroma at build time, so that stays
true for posts.

`/projects` fetches repos from the GitHub API on demand, cached five minutes, with
`s-maxage=300` so a CDN absorbs the calls. New public repos appear on their own.
The homepage "Projects" section is a separate hand-curated list in `site.go`.

## Layout

```
main.go       routing, static files, GitHub fetching, list filtering
render.go     template setup, prerender pass, pagination
content.go    markdown posts: frontmatter, highlighting
site.go       hand-written content: experience, projects, socials
icons.go      inlined SVGs
templates/    one file per route; layout.gohtml holds the document
static/       site.css, htmx.min.js, images
content/      posts
```

## Adding a post

Drop a `.md` file in `content/` with YAML frontmatter (`published: false` hides
it) and redeploy.

## Credits

[htmx](https://htmx.org) (BSD-3-Clause), [goldmark](https://github.com/yuin/goldmark)
and [chroma](https://github.com/alecthomas/chroma) (MIT), [Lucide](https://lucide.dev) icons (ISC).