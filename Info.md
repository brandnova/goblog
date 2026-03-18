# GoBlog — Info

---

## Deployment on Leapcell

GoBlog is deployed on [Leapcell](https://leapcell.io) using a free PostgreSQL
database and a serverless web service.

### 1. Create the PostgreSQL Database

1. Sign up or log in at [leapcell.io](https://leapcell.io)
2. Dashboard → **New Resource** → **PostgreSQL**
3. Give it a name (e.g. `goblog-db`) and confirm
4. Copy the **Connection String** once provisioned:
   `postgres://username:password@host:5432/dbname`

The schema is created automatically on first startup via `initDB()` in `db.go`.

### 2. Create the Web Service

1. Dashboard → **New Service** → **Web Service**
2. Connect GitHub and select your repo
3. Build settings:

| Field | Value |
|---|---|
| **Runtime** | Go |
| **Build Command** | `go build -o goblog .` |
| **Start Command** | `./goblog` |

### 3. Set Environment Variables

| Key | Value |
|---|---|
| `DATABASE_URL` | connection string from Step 1 |
| `PORT` | `8080` |

### 4. Deploy

Click **Deploy**. A successful build ends with:
```
Database ready.
Server running at http://localhost:8080
```
You receive a free `.leapcell.app` subdomain. Every `git push` to `main`
triggers an automatic redeploy.

### Troubleshooting

**Build fails with missing module errors** — run `go mod tidy` locally,
commit `go.mod` and `go.sum`, then push again.

**`DATABASE_URL environment variable is not set`** — check for typos or extra
spaces in the Leapcell environment settings panel.

---

## Cover Image Uploads — Leapcell Object Storage

### Why local file storage doesn't work on Leapcell

Leapcell's serverless plan has a read-only filesystem (except `/tmp`). Writing
to `static/uploads/` silently fails — the path gets stored in the database but
no file is ever written, so the image returns a 404.

This is not a Leapcell quirk — it is how all serverless platforms work (Vercel,
Render, Fly.io, Railway all behave the same way). The correct solution is to
store uploaded files in a dedicated object storage service.

### Leapcell Object Storage (Beta)

Leapcell provides free S3-compatible object storage alongside its web services.

**Free tier limits:** 100 MB storage · 5,000 files · CDN included · 12-hour
cache TTL. Hard limits — cannot be increased. Sufficient for a personal blog.

**Creating a bucket:**
1. Dashboard → **New Resource** → **Create Object Storage (+CDN) (Beta)**
2. Copy the bucket name, endpoint, access key, secret key, and CDN base URL

**⚠️ Never commit credentials to Git.**

### Environment Variables

Add to your Leapcell service's Environment panel (note: `LEAPCELL_` prefix is
blocked by Leapcell, so we use `OBJ_`):

| Key | Value |
|---|---|
| `OBJ_ENDPOINT` | `https://objstorage.leapcell.io` |
| `OBJ_BUCKET` | your bucket name |
| `OBJ_ACCESS_KEY` | your access key ID |
| `OBJ_SECRET_KEY` | your secret access key |
| `OBJ_CDN_URL` | `https://subdomain.leapcellobj.com/bucket-name` |

Leave these unset in local `.env` to keep using local disk during development.

### How the Toggle Works

| `OBJ_ENDPOINT` set? | Where files go | URL in DB |
|---|---|---|
| No (local dev) | `static/uploads/` on disk | `/static/uploads/filename.jpg` |
| Yes (production) | Leapcell Object Storage | CDN URL |

To revert: remove `OBJ_ENDPOINT` from the environment panel and redeploy.
`storage.go` can stay in the repo — without the env var it is inert.

### Fixing Broken Cover Images

Posts created before adding object storage have broken image URLs in the
database. Edit each post from the dashboard and re-upload the cover image —
the new upload goes to object storage and the URL updates automatically.

### Switching to Cloudflare R2 (if Leapcell storage is too limited)

Cloudflare R2 offers 10 GB free with zero egress fees. It uses the same S3
API — only the endpoint and credentials change in `storage.go`. The rest of
the codebase is unchanged.

---

## TODO — Project Improvements

### ✅ Completed

**Core platform**
- [x] Auth — register, login, logout with bcrypt + server-side sessions
- [x] Posts — create, edit, delete, publish with Markdown support
- [x] Draft / Published status
- [x] Tags — comma-separated, filtered listing pages
- [x] Unique slugs — UUID suffix prevents all collisions
- [x] Slug permanent on edit — never regenerates, links never break
- [x] Cover images — local disk (dev) / Leapcell Object Storage (prod)
- [x] Read time estimation
- [x] Markdown rendering + plain-text excerpt stripping

**Multi-user**
- [x] Profile pages at `/u/{username}` with bio and display name
- [x] Post URLs scoped to author — `/u/{username}/{slug}`
- [x] Settings page — name, bio, email, password change (split forms)
- [x] Dashboard with post metrics (reactions, bookmarks, read time)

**Discovery & interaction**
- [x] Bookmarks — HTMX toggle, reading list page
- [x] Reactions (likes) — HTMX toggle, count visible to all
- [x] Live search — HTMX, debounced, fixed rendering bug
- [x] Infinite scroll pagination — index, profile, tag, bookmarks, search

**Infrastructure**
- [x] PostgreSQL with `$N` placeholders throughout
- [x] Session cleanup goroutine — purges expired rows every 24 hours
- [x] CSRF protection — double-submit cookie pattern on all POST forms
- [x] Page titles — dynamic per-page via `renderWithTitle()`
- [x] Custom 404 page
- [x] Flash messages after create/update post
- [x] `AttachTags()` — single-query tag attachment for all list pages
- [x] Reaction counts on list pages
- [x] Post updated timestamp on detail page
- [x] Settings name grid — proper CSS class, no inline `<style>` tag
- [x] Responsive mobile nav with hamburger menu
- [x] `leapcell.io` deployment

---

### 🔧 Current Phase — Remaining

**UX / Polish**
- [ ] **Search not working in production** — the HTMX live search was fixed
  locally (`renderPartial` bug) but should be verified on the deployed site
  after the latest push
- [ ] **Loading state on HTMX delete** — the dashboard delete button has no
  visual feedback between click and row removal; add `htmx-indicator` or a
  brief opacity change via CSS
- [ ] **Reaction/bookmark loading state** — buttons show no feedback on slow
  connections; HTMX's `htmx-request` class can drive a CSS loading state
  without any JavaScript
- [ ] **Settings nav indicator** — no visual cue that you're on the settings
  page; the current page link should have a distinct colour or underline

**Correctness**
- [ ] **Tags on dashboard** — `GetPostsByUser` fetches tags via subquery but
  the dashboard template should show tag pills per post row (the data is
  there, template just doesn't render them yet)
- [ ] **Broken cover image URLs** — posts created before object storage was
  added have stale local paths in the database; add a note or admin tool
  to identify and fix them

---

### 🚀 Next Phase — Features

**Content & discovery**
- [ ] **Comments** — `comments` table, HTMX partial form, prepend to list
  on submit; no page reload needed
- [ ] **RSS feed** — `/feed.rss` using `encoding/xml` (stdlib only, ~50 lines)
- [ ] **Related posts** — 2–3 posts by same author or overlapping tags shown
  at the bottom of each post detail page; one SQL query with `LIMIT 3`
- [ ] **Post updated timestamp on list cards** — currently only on detail page
- [ ] **Full-text search** — replace `ILIKE` with PostgreSQL `tsvector` for
  relevance ranking, stemming, and phrase queries

**Social**
- [ ] **Follow system** — `follows` table, personalised feed at `/feed`
  showing only posts from people you follow; transforms the platform
  from "many blogs" to "a network"
- [ ] **User mentions** — `@username` in post bodies links to profiles
- [ ] **Share counts / external share buttons** — Twitter/X, copy link

**Platform**
- [ ] **Email verification on registration** — goroutine sends email in the
  background via Resend or Postmark; `verified` column on users
- [ ] **Rate limiting on login/register** — in-memory counter per IP, 5-minute
  window; prevents brute-force without external dependency
- [ ] **Input length validation** — server-side max lengths on title, body,
  bio; currently unlimited
- [ ] **Admin panel** — `/admin` behind role check; moderate content,
  view platform stats, ban users

**Developer experience**
- [ ] **`air` live reload** — `go install github.com/air-verse/air@latest`;
  replaces manual `go run .` restarts during development
- [ ] **`chi` router** — replace stdlib mux once routes grow; enables
  middleware per group, which will matter when admin routes are added
- [ ] **Testing** — model-level tests with a test database; `go test ./...`

---

## Go Concepts Worth Learning Next

**Error handling patterns** — custom error types to distinguish 404, 403, and
500 cleanly across handlers instead of inline `http.Error` calls everywhere.

**Interfaces** — Go's implicit interface model is how `http.Handler` works.
Understanding it unlocks middleware composition and makes the codebase easier
to test.

**Testing** — `go test` is built in. Start with model-level tests: spin up a
test PostgreSQL database, run queries, assert results. No framework needed.

**`chi` router** — once route groups with per-group middleware are needed
(e.g. an admin group), `chi` is the clearest upgrade path from the stdlib mux.
All existing handlers work unchanged.

**`air`** — live reloading dev server. Watches files and restarts `go run .`
automatically on save. Install with `go install github.com/air-verse/air@latest`.

---

## The Django → Go Mental Model

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
| `Migrations` | `CREATE TABLE IF NOT EXISTS` + `ALTER TABLE IF NOT EXISTS` |
| `mark_safe()` | `template.HTML(...)` |
| Custom template filter | `template.FuncMap` |
| `python-decouple` / `django-environ` | `godotenv` + `os.Getenv()` |
| `django-storages` | `storage.go` with env-var toggle |
| `paginator.page(n)` | `LIMIT $1 OFFSET $2` + HTMX infinite scroll |