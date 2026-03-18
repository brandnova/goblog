# GoBlog — Project Info

GoBlog is a multi-user blog platform built in Go. The goal is a complete,
self-hosted alternative to Blogger — simple in architecture, but robust,
secure, fully moderated, and engaging. No JavaScript frameworks. No ORM.
Server-rendered HTML, HTMX for interactivity, PostgreSQL for storage.

---

## Deployment — Leapcell

GoBlog is deployed on [Leapcell](https://leapcell.io) using a free PostgreSQL
database and a serverless web service.

### Setup Steps

**1. Create the PostgreSQL database**
Dashboard → New Resource → PostgreSQL. Copy the connection string once
provisioned: `postgres://user:pass@host:5432/dbname`

**2. Create the Web Service**
Dashboard → New Service → Web Service → connect GitHub repo.

| Field | Value |
|---|---|
| Runtime | Go |
| Build Command | `go build -o goblog .` |
| Start Command | `./goblog` |

**3. Set environment variables**

| Key | Value |
|---|---|
| `DATABASE_URL` | connection string from step 1 |
| `PORT` | `8080` |

**4. Deploy**
Click Deploy. Every `git push` to `main` triggers an automatic redeploy.
A successful build ends with `Database ready.`

### Troubleshooting

- **Missing module errors** — run `go mod tidy` locally, commit `go.mod` and
  `go.sum`, push again
- **DATABASE_URL not set** — check for typos or spaces in the env panel
- **WebSocket connections drop** — Leapcell's free plan is serverless;
  persistent connections require the Plus plan or migrating to Railway

---

## Cover Image Uploads — Leapcell Object Storage

Leapcell's serverless plan has a read-only filesystem. Writing to
`static/uploads/` silently fails in production — the path is saved to the
database but no file is written, so images return 404.

The solution is Leapcell's built-in S3-compatible object storage (beta).

### Setup

Dashboard → New Resource → Create Object Storage (+CDN) (Beta).
Copy the bucket name, endpoint, access key, secret key, and CDN base URL.

**⚠️ Never commit credentials to Git.**

Add to your Leapcell service's Environment panel (`LEAPCELL_` prefix is
blocked by Leapcell — use `OBJ_` instead):

| Key | Value |
|---|---|
| `OBJ_ENDPOINT` | `https://objstorage.leapcell.io` |
| `OBJ_BUCKET` | your bucket name |
| `OBJ_ACCESS_KEY` | your access key ID |
| `OBJ_SECRET_KEY` | your secret access key |
| `OBJ_CDN_URL` | `https://subdomain.leapcellobj.com/bucket-name` |

Leave these unset locally to keep using local disk during development.

### Free Tier Limits

100 MB storage · 5,000 files · CDN included · 12-hour cache TTL.
Hard limits — cannot be increased. Sufficient for a personal/small blog.

### Toggle Table

| `OBJ_ENDPOINT` set? | Where files go | URL in DB |
|---|---|---|
| No (local dev) | `static/uploads/` on disk | `/static/uploads/filename.jpg` |
| Yes (production) | Leapcell Object Storage | CDN URL |

To revert: remove `OBJ_ENDPOINT` from the environment panel and redeploy.

### Cloudflare R2 (if Leapcell storage becomes insufficient)

Cloudflare R2 offers 10 GB free with zero egress fees. It is a drop-in
replacement — only the endpoint and credentials in `storage.go` change.

---

## Project Phases — Start to Launch

### Phase 0 — Foundation ✅
*The working skeleton. Auth, posts, basic UI.*

- [x] Go project structure — `main.go`, `handlers/`, `models/`, `templates/`
- [x] PostgreSQL connection via `sqlx` + `lib/pq`
- [x] Auto schema creation on startup — no migration tool needed
- [x] User registration and login with bcrypt password hashing
- [x] Server-side sessions — UUID tokens in `sessions` table
- [x] Session cookie — `HttpOnly`, `SameSite=Lax`
- [x] `AuthMiddleware` — attaches user to every request context
- [x] `RequireLogin` — per-route auth guard
- [x] Posts — create, edit, delete with Markdown rendering
- [x] Draft / Published status
- [x] Unique slugs — `title-slug-uuid8` format, permanent after creation
- [x] Base HTML layout — Playfair Display + Source Serif 4 + JetBrains Mono
- [x] Tailwind CSS compiled locally via Node subdirectory
- [x] Responsive mobile nav with hamburger menu
- [x] Custom 404 page
- [x] Leapcell deployment

---

### Phase 1 — Multi-User Platform ✅
*Every user has their own space. Discovery begins.*

- [x] Profile pages at `/u/{username}` with bio and display name
- [x] Post URLs scoped to author — `/u/{username}/{slug}`
- [x] Settings page — display name, bio, email, split password form
- [x] Dashboard — user's own posts with reaction + bookmark counts
- [x] Tags — comma-separated, stored in `tags` + `post_tags` tables
- [x] Tag filter pages at `/tag/{name}`
- [x] Cover images — local disk (dev) / Leapcell Object Storage (prod)
- [x] Read time estimation — word count ÷ 200 wpm
- [x] Markdown rendering + plain-text excerpt stripping
- [x] `AttachTags()` — single-query tag attachment for all list pages

---

### Phase 2 — Interaction & Discovery ✅
*Users engage with content. The platform feels alive.*

- [x] Bookmarks — `bookmarks` table, HTMX toggle, reading list page
- [x] Reactions (likes) — `reactions` table, HTMX toggle, count on all pages
- [x] Live search — HTMX debounced, `renderPartial` bug fixed
- [x] Infinite scroll pagination — index, profile, tag, bookmarks, search
- [x] Flash messages — `?new=1` and `?saved=1` after post create/update
- [x] Post updated timestamp on detail page
- [x] Reaction counts on all list pages
- [x] Tags rendering on all list pages

---

### Phase 3 — Security & Correctness ✅
*The platform is safe and reliable before it gets more users.*

- [x] CSRF protection — double-submit cookie on all POST forms
- [x] `CSRFMiddleware` in the request chain
- [x] Session cleanup goroutine — purges expired rows every 24 hours
- [x] Slug collision prevention — UUID suffix, slug never changes on edit
- [x] Reaction/bookmark HTMX endpoints exempt from CSRF (SameSite=Lax)
- [x] Dynamic page titles via `renderWithTitle()`
- [x] Active nav link indicator — `.nav-link-active` class via `{{.Path}}`
- [x] `hasPrefix` registered in `templateFuncs` for profile path matching
- [x] Settings name grid — proper CSS class, no inline `<style>` tag
- [x] `AttachTags()` called in Dashboard handler

---

### Phase 4 — Content & Discovery 🔧 *current*
*The platform becomes a place people want to read.*

- [ ] **Comments** — `comments` table, HTMX partial, prepend on submit
- [ ] **Related posts** — 3–5 by same author or overlapping tags at bottom
  of post detail page; one query with `LIMIT 5`
- [ ] **RSS feed** — `/feed.rss` via `encoding/xml`, no third-party lib
- [ ] **Post reaction count on detail page** — the count is inside the
  toggle button; add it also as a plain visible count in the meta row
  for logged-out visitors
- [ ] **Full-text search** — replace `ILIKE` with PostgreSQL `tsvector`
  for relevance ranking, stemming, and phrase queries
- [ ] **`air` live reload** — `go install github.com/air-verse/air@latest`;
  `.air.toml` in project root; replaces manual `go run .`
- [ ] **`chi` router** — replace stdlib mux when admin routes are added;
  enables per-group middleware

---

### Phase 5 — Social Layer
*Users connect with each other. The platform has a network effect.*

- [ ] **Follow system** — `follows` table (`follower_id`, `following_id`);
  personalised feed at `/feed` showing posts from followed authors;
  follow/unfollow button on profile pages
- [ ] **Follower/following counts** on profile pages
- [ ] **@username mentions** — in post bodies and comments; links to profile
- [ ] **External share buttons** — Twitter/X and copy-link on post detail
- [ ] **Post view counter** — simple increment on every detail page load;
  shown on dashboard alongside reactions and bookmarks

---

### Phase 6 — Platform Hardening
*The platform is safe to open to the public.*

- [ ] **Email verification on registration** — `verified` column on users;
  token in `email_tokens` table; goroutine sends via Resend/Postmark;
  unverified users can write drafts but cannot publish
- [ ] **Rate limiting on login/register** — in-memory counter per IP,
  5-minute window; no external dependency
- [ ] **Input length validation** — server-side max lengths on title (200),
  body (50 000), bio (300), comment (2 000); return 400 with field error
- [ ] **In-app notifications** — `notifications` table; triggers for: new
  reaction on your post, new comment on your post, new follower, @mention;
  bell icon in nav with unread count badge; mark-all-read endpoint.
  Delivered in real-time via a WebSocket channel backed by Redis pub/sub —
  This also opens the door to making other parts of the platform real-time 
  (live comment feeds, reaction count updates) by multiplexing over the same 
  WebSocket connection.
- [ ] **Report content** — users can flag posts or comments as spam or abuse;
  flagged items appear in the admin queue

---

### Phase 7 — Admin & Moderation
*The platform owner can keep things healthy.*

- [ ] **Admin panel** — `/admin` behind `role = "admin"` check on the user
- [ ] **User management** — list all users, view profile, suspend/ban account
- [ ] **Content moderation** — list flagged posts and comments; approve,
  remove, or dismiss flags; remove any post or comment platform-wide
- [ ] **Site settings** — set site name, tagline, registration open/closed,
  maintenance mode; stored in a `site_settings` table
- [ ] **Platform stats** — total users, posts, comments, reactions; daily
  new signups; shown on admin dashboard
- [ ] **Role system** — `role` column on users: `"user"` (default),
  `"moderator"` (can remove content), `"admin"` (full access)

---

### Phase 8 — Polish & Launch Readiness
*Everything a real product needs before going public.*

- [ ] **SEO meta tags** — Open Graph and Twitter Card tags on every page;
  post detail page gets description from excerpt, image from cover
- [ ] **Sitemap** — `/sitemap.xml` listing all published posts; helps search
  engine indexing
- [ ] **`robots.txt`** — allow all crawlers on public content; block `/admin`
- [ ] **`chi` router** — migrate from stdlib mux for cleaner route groups
- [ ] **Testing** — model-level tests with a test PostgreSQL database;
  `go test ./...`; start with auth and post creation flows
- [ ] **Error pages** — custom 403 and 500 pages matching the site design
- [ ] **Graceful shutdown** — `os.Signal` listener calls `db.Close()` and
  drains in-flight requests before exiting
- [ ] **Structured logging** — replace `log.Printf` with `slog` (Go 1.21+)
  for JSON-formatted logs that Leapcell's log panel can parse
- [ ] **README** — setup instructions, env var reference, deployment guide

---

## Architecture Reference

### Request Flow
```
Browser → CSRFMiddleware → AuthMiddleware → mux → handler → model → PostgreSQL
                                                         ↓
                                                    template → HTML → browser
```

### Middleware Chain (outermost first)
1. `CSRFMiddleware` — reads/generates CSRF cookie, attaches token to context
2. `AuthMiddleware` — reads session cookie, attaches user to context
3. `mux` — routes request to handler

### Template Data Shape
Every `render()` / `renderWithTitle()` call injects:
```go
{
    Data      any     // page-specific data
    User      *User   // nil if logged out
    CSRFToken string  // from CSRF cookie
    PageTitle string  // empty → "GoBlog"; set → "Title — GoBlog"
    Path      string  // r.URL.Path for nav active state
}
```

### File Layout
```
goblog/
├── main.go              — entry point, routes, middleware chain
├── db.go                — DB init, schema, migrations, session cleanup
├── storage.go           — object storage toggle (local vs Leapcell S3)
├── handlers/
│   ├── helpers.go       — render(), renderWithTitle(), renderPartial(), templateFuncs
│   ├── middleware.go    — CSRF, Auth, RequireLogin, sessions
│   ├── auth.go          — Register, Login, Logout, Settings
│   └── posts.go         — all post/tag/search/bookmark/reaction handlers
├── models/
│   ├── user.go          — User struct, CRUD, auth helpers
│   └── post.go          — Post/Tag structs, all queries, AttachTags, pagination
├── templates/
│   ├── base.html        — layout, CSS tokens, nav, CSRF in forms
│   ├── index.html       — homepage with live search
│   ├── post.html        — post detail with reactions, bookmarks, flash
│   ├── form.html        — create/edit post form
│   ├── dashboard.html   — author dashboard
│   ├── profile.html     — public profile
│   ├── bookmarks.html   — reading list
│   ├── tag.html         — tag-filtered posts
│   ├── settings.html    — profile + password forms
│   ├── login.html
│   ├── register.html
│   ├── 404.html
│   └── partials/
│       ├── post_list.html      — infinite scroll post rows
│       └── search_results.html — HTMX search dropdown
└── static/
    ├── css/tailwind.css
    └── js/htmx.min.js
```

### Database Schema
```
users         — id, username, email, password_hash, first_name, last_name, bio, created_at
posts         — id, user_id, title, slug, body, cover_image, status, created_at, updated_at
tags          — id, name (unique)
post_tags     — post_id, tag_id (composite PK, CASCADE on delete)
sessions      — token (PK), user_id, expires_at
bookmarks     — user_id, post_id (composite PK)
reactions     — user_id, post_id (composite PK)
```

Upcoming tables (Phase 4+):
```
comments      — id, post_id, user_id, body, created_at
follows       — follower_id, following_id (composite PK)
notifications — id, user_id, type, actor_id, post_id, comment_id, read, created_at
email_tokens  — id, user_id, token, expires_at
site_settings — key, value
```

---

## Django → Go Mental Model

| Django | Go equivalent in GoBlog |
|---|---|
| `SessionMiddleware` | `AuthMiddleware` |
| `request.user` | `CurrentUser(r)` via context |
| `@login_required` | `RequireLogin()` wrapper |
| `CsrfViewMiddleware` + `{% csrf_token %}` | `CSRFMiddleware` + `{{.CSRFToken}}` |
| `make_password()` / `check_password()` | `bcrypt.GenerateFromPassword()` / `CompareHashAndPassword()` |
| `User.objects.create_user()` | `models.CreateUser()` |
| `Post.objects.filter(status='published')` | `models.GetAllPublished()` |
| `post.tags.set(tags)` | `syncTags()` |
| `prefetch_related('tags')` | `models.AttachTags()` |
| `get_object_or_404()` | `getPostForEdit()` with nil check |
| `UserPassesTestMixin` | ownership check in `getPostForEdit()` |
| `slugify()` | `models.Slugify()` + `UniqueSlug()` |
| `{% static 'file' %}` | `/static/file` via `http.FileServer` |
| `{% extends %}` / `{% block %}` | `template.ParseFiles` + `{{block}}` |
| `render(request, template, context)` | `render(w, r, data, template)` |
| `@shared_task` (Celery) | `go func()` — goroutine |
| Migrations | `CREATE TABLE IF NOT EXISTS` + `ALTER TABLE IF NOT EXISTS` |
| `mark_safe()` | `template.HTML(...)` |
| Custom template filter | `template.FuncMap` |
| `python-decouple` | `godotenv` + `os.Getenv()` |
| `django-storages` | `storage.go` with env-var toggle |
| `paginator.page(n)` | `LIMIT $1 OFFSET $2` + HTMX infinite scroll |
| `django.contrib.admin` | custom `/admin` (Phase 7) |
| `signals.post_save` | goroutine launched from handler |