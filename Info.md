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
replacement — only the endpoint and credentials in `handlers/storage.go` change.

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
    Path      string  // r.URL.Path — for nav active state
    UserPath  string  // "/u/{username}" — exact match for profile nav link
}
```

### File Layout
```
goblog/
├── main.go                   — entry point, routes, middleware chain
├── db.go                     — DB init, schema, migrations, session cleanup
├── go.mod / go.sum
├── .air.toml                 — Air live reload config (go install air@latest)
├── .env.example
├── .gitignore
├── Info.md
├── README.md
├── LICENSE
├── handlers/
│   ├── auth.go               — Register, Login, Logout, Settings handlers
│   ├── helpers.go            — render(), renderWithTitle(), renderPartial(), templateFuncs
│   ├── middleware.go         — CSRFMiddleware, AuthMiddleware, RequireLogin, sessions
│   ├── posts.go              — all post/tag/search/bookmark/reaction handlers
│   └── storage.go            — object storage toggle (local disk vs Leapcell S3)
├── models/
│   ├── post.go               — Post/Tag structs, all queries, AttachTags, pagination
│   └── user.go               — User struct, CRUD, auth helpers
├── static/
│   ├── css/
│   │   ├── goblog.css        — all custom styles (extracted from base.html)
│   │   └── tailwind.css      — compiled Tailwind output
│   └── js/
│       └── htmx.min.js
├── tailwind/                 — Tailwind compilation toolchain (Node, not served)
│   ├── src/styles.css
│   ├── tailwind.config.js
│   ├── postcss.config.js
│   ├── package.json
│   └── package-lock.json
└── templates/
    ├── base.html             — layout, nav, CSRF forms, footer
    ├── index.html            — homepage with live search + infinite scroll
    ├── post.html             — post detail with reactions, bookmarks, flash
    ├── form.html             — create/edit post form with tag counter
    ├── dashboard.html        — author dashboard with metrics
    ├── profile.html          — public profile page
    ├── bookmarks.html        — reading list
    ├── tag.html              — tag-filtered post listing
    ├── settings.html         — profile + password forms
    ├── login.html
    ├── register.html
    ├── 404.html
    └── partials/
        ├── post_list.html    — shared infinite scroll post rows + sentinel
        └── search_results.html — HTMX search results dropdown
```

### Database Schema

**Current tables:**
```
users         — id, username, email, password_hash, first_name, last_name,
                bio, created_at
posts         — id, user_id, title, slug, body, cover_image, status,
                created_at, updated_at
tags          — id, name (unique)
post_tags     — post_id, tag_id (composite PK, CASCADE on delete)
sessions      — token (PK), user_id, expires_at
bookmarks     — user_id, post_id (composite PK)
reactions     — user_id, post_id (composite PK)
```

**Phase 3.5 additions:**
```
users         — + avatar_url, location, website, twitter, github, instagram
site_settings — key (PK), value
```

**Phase 4+ additions:**
```
comments      — id, post_id, user_id, body, created_at
```

**Phase 5+ additions:**
```
follows       — follower_id, following_id (composite PK)
post_views    — post_id, ip_hash, viewed_at (dedup within 24h)
```

**Phase 6+ additions:**
```
notifications — id, user_id, type, actor_id, post_id, comment_id,
                read, created_at
email_tokens  — id, user_id, token, expires_at
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
| `django-storages` | `handlers/storage.go` with env-var toggle |
| `paginator.page(n)` | `LIMIT $1 OFFSET $2` + HTMX infinite scroll |
| `django.contrib.admin` | custom `/admin` (Phase 7) |
| `signals.post_save` | goroutine launched from handler |
| `SiteConfig` / `django-constance` | `site_settings` table + `GetSetting()` helper |