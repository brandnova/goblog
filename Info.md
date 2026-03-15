# GoBlog — What's Next

A reference doc covering deployment options, feature ideas, and Go concepts
worth knowing as you grow this project.

---

## Deployment Platforms for Go Apps

### Free Tiers (No Credit Card Required)

**Leapcell** — `leapcell.io`
The friendliest free tier for Go specifically. Connects to GitHub, builds your
Go binary automatically on push, and only charges for actual usage — meaning
it costs nothing when idle. Also provides free PostgreSQL and Redis. The most
straightforward option for this project.

```
1. Push your repo to GitHub
2. Sign up at leapcell.io
3. New Service → connect GitHub repo → set build command: go build -o goblog .
4. Set start command: ./goblog
5. Deploy — you get a .leapcell.dev subdomain immediately
```

**Koyeb** — `koyeb.com`
Free tier includes one always-on web service with a `.koyeb.app` subdomain.
Supports git-driven deployment — push to GitHub and Koyeb rebuilds
automatically. Has first-class Go support with automatic build detection.

```
1. Sign up at koyeb.com
2. Create App → GitHub → select your repo
3. Koyeb detects Go, sets build and run commands automatically
4. Deploy
```

**Render** — `render.com`
Well-known platform with a free tier for web services (note: free instances
spin down after 15 minutes of inactivity and take ~30 seconds to wake up,
which is fine for a personal blog but noticeable). Supports Go natively.

```
1. Sign up at render.com
2. New → Web Service → connect GitHub repo
3. Build command: go build -o goblog .
4. Start command: ./goblog
5. Deploy
```

**Fly.io** — `fly.io`
Slightly more technical but very powerful. Runs your app as a container
globally. Requires a Dockerfile (see below) and their CLI tool. Free tier
allows up to 3 shared-CPU VMs. No cold starts.

```bash
# Install Fly CLI
curl -L https://fly.io/install.sh | sh

# From your project root
fly launch        # auto-detects Go, creates fly.toml
fly deploy        # builds and deploys
```

---

### Switching from SQLite to PostgreSQL for Deployment

SQLite works perfectly for development but most cloud platforms don't provide
persistent disk storage on free tiers — meaning your `blog.db` file disappears
on every redeploy. For production, swap to PostgreSQL.

Leapcell and Render both offer free managed Postgres. The code change is
smaller than you'd think because all your SQL is already explicit:

```bash
go get github.com/lib/pq    # PostgreSQL driver
```

In `db.go`, change two things:

```go
// Before (SQLite)
import _ "github.com/mattn/go-sqlite3"
db, err = sqlx.Connect("sqlite3", "blog.db")

// After (PostgreSQL)
import _ "github.com/lib/pq"
db, err = sqlx.Connect("postgres", os.Getenv("DATABASE_URL"))
```

Also replace SQLite-specific syntax in your queries:
- `?` placeholders → `$1, $2, $3` (PostgreSQL uses numbered params)
- `INSERT OR IGNORE` → `INSERT ... ON CONFLICT DO NOTHING`
- `AUTOINCREMENT` → `SERIAL` or `BIGSERIAL`

---

### Dockerfile (Required for Fly.io, Optional for Others)

A two-stage Docker build keeps the final image small — the builder stage
compiles the binary, the runner stage only contains the binary itself.

```dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o goblog .

# Stage 2: Run
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/goblog .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static
EXPOSE 8080
ENTRYPOINT ["./goblog"]
```

Note: if you switch to PostgreSQL, you can drop `gcc musl-dev` from the builder
stage since `go-sqlite3` (which uses CGo) is no longer a dependency.

---

## Feature Ideas — Turning GoBlog into a "Blogger for Everyone"

You mentioned wanting something like Blogger but imagined differently — a
platform where individuals each manage their own content. Here's a roadmap
from simplest to most ambitious.

### Tier 1 — Low Effort, High Impact

**Profile pages** — `/u/{username}`
Each user gets a public page listing all their published posts. One new route,
one new query (`GetPostsByUser` already exists), one new template.

**Post slugs scoped to users** — `/u/{username}/{slug}`
Right now two users can't publish a post with the same slug. Scoping URLs to
the user fixes this and makes the multi-user nature of the platform explicit
in the URL — just like Medium's `/@username/post-title` pattern.

**Reading list / bookmarks**
A `bookmarks` join table (`user_id`, `post_id`). Users can save posts to read
later. One button with an HTMX POST to toggle it — no page reload.

**Post reactions** (likes/claps)
Similar to bookmarks — a `reactions` table. The HTMX pattern makes this
genuinely easy: the button fires `hx-post="/post/{id}/react"`, the handler
toggles the reaction and returns an updated count fragment.

---

### Tier 2 — Medium Effort

**Comments**
A `comments` table (`id`, `post_id`, `user_id`, `body`, `created_at`).
HTMX makes this elegant — the comment form posts to `/post/{id}/comment`,
the handler saves the comment and returns just the new comment HTML fragment,
HTMX prepends it to the comments list. No page reload, no JavaScript.

```go
type Comment struct {
    ID        int       `db:"id"`
    PostID    int       `db:"post_id"`
    UserID    int       `db:"user_id"`
    Body      string    `db:"body"`
    CreatedAt time.Time `db:"created_at"`
    AuthorName string   `db:"author_name"`
}
```

**Email verification on registration**
Use Go's `net/smtp` package (stdlib) or a transactional email service like
Resend or Postmark. Store a `verified` boolean on the user and an
`email_tokens` table. A goroutine sends the email in the background — the
user doesn't wait.

**Pagination**
The index page will become slow with hundreds of posts. Standard SQL pagination:
```go
// Page 1: LIMIT 10 OFFSET 0
// Page 2: LIMIT 10 OFFSET 10
func GetAllPublished(db *sqlx.DB, page, perPage int) ([]Post, error) {
    offset := (page - 1) * perPage
    db.Select(&posts, `... LIMIT ? OFFSET ?`, perPage, offset)
}
```

**RSS Feed** — `/feed.rss`
Go's `encoding/xml` package makes this straightforward. An RSS feed is just
an XML document. No third-party package needed. Many personal blog readers
still use RSS, and it's a legitimately impressive feature to add in ~50 lines.

---

### Tier 3 — Ambitious

**Follow system**
A `follows` table (`follower_id`, `following_id`). A personalised feed at `/feed`
shows posts only from people you follow — like Twitter/Medium's following feed.
This is the feature that transforms the platform from "many blogs" to "a network."

**Full-text search with SQLite FTS5**
SQLite has a built-in full-text search extension (FTS5) that is dramatically
faster and smarter than `LIKE %query%`. It supports ranking by relevance,
prefix matching, and phrase queries. No extra package needed.

```sql
-- One-time setup
CREATE VIRTUAL TABLE posts_fts USING fts5(title, body, content=posts, content_rowid=id);

-- Query
SELECT * FROM posts_fts WHERE posts_fts MATCH 'golang web development';
```

**Image storage on S3/R2**
Right now cover images are saved to `static/uploads/` on disk. On cloud
platforms this folder is ephemeral. Cloudflare R2 is S3-compatible and has a
generous free tier (10GB storage, no egress fees). The Go AWS SDK works with R2
with a single endpoint change.

**Admin panel**
An `/admin` section protected by a role check (`user.Role == "admin"`). Lets
an admin moderate content, ban users, and view platform-wide stats. Add a
`role` column to the `users` table (`"user"` or `"admin"`).

---

## Go Concepts Worth Learning Next

**Error handling patterns**
Go's explicit error handling is verbose but powerful. The next step is learning
to create custom error types so you can distinguish between "not found",
"forbidden", and "server error" cleanly across your handlers.

```go
type AppError struct {
    Code    int    // HTTP status code
    Message string
}
func (e *AppError) Error() string { return e.Message }

var ErrNotFound  = &AppError{404, "not found"}
var ErrForbidden = &AppError{403, "forbidden"}
```

**Interfaces**
Go's interfaces are implicit — a type implements an interface just by having
the right methods, no `implements` keyword. This is how `http.Handler` works,
and understanding it unlocks a lot of Go's design patterns.

**Testing**
Go has a built-in test runner (`go test`). Writing tests for your model
functions (the SQL queries) is straightforward and genuinely useful. The
testing philosophy is similar to Django's `TestCase` but without a test
database — you'd spin up an in-memory SQLite database per test.

```bash
go test ./...          # run all tests
go test -v ./models/   # verbose output for the models package
```

**`chi` router**
Once your route list grows, the stdlib mux starts to feel limited — no route
groups, no middleware per group, no named URL reversal. `chi` is the most
Django-like Go router: lightweight, composable, and it uses the same
`http.Handler` interface so all your existing handlers work unchanged.

```go
r := chi.NewRouter()
r.Use(middleware.Logger)        // global middleware
r.Group(func(r chi.Router) {
    r.Use(RequireLogin)         // group-level middleware
    r.Get("/new", NewPostPage)
    r.Post("/new", CreatePost)
})
```

**`air` — live reload**
The Go equivalent of Django's auto-reloading dev server. Right now you restart
`go run .` manually after every change. Air watches your files and restarts
automatically.

```bash
go install github.com/air-verse/air@latest
air    # run instead of "go run ." in development
```

**Configuration with environment variables**
Right now settings like the port and database path are hardcoded. The standard
Go approach is `os.Getenv()`. For local dev, a `.env` file loaded with
`github.com/joho/godotenv` gives you the same workflow as Django's
`python-decouple` or `django-environ`.

```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
log.Fatal(http.ListenAndServe(":"+port, handler))
```

---

## The Django → Go Mental Model, Fully Resolved

After building this project, here's the complete picture of what Django was
doing for you automatically, and what you've now written yourself:

| Django | What you built in Go |
|---|---|
| `SessionMiddleware` | `AuthMiddleware` in `middleware.go` |
| `AuthenticationMiddleware` | The user lookup inside `AuthMiddleware` |
| `request.user` | `CurrentUser(r)` via request context |
| `@login_required` | `RequireLogin()` wrapper |
| `make_password()` | `bcrypt.GenerateFromPassword()` |
| `check_password()` | `bcrypt.CompareHashAndPassword()` |
| `User.objects.create_user()` | `models.CreateUser()` |
| `Post.objects.filter(status='published')` | `models.GetAllPublished()` |
| `post.tags.set(tags)` | `syncTags()` |
| `get_object_or_404()` | `getPostForEdit()` with nil check |
| `UserPassesTestMixin` | The ownership check in `getPostForEdit()` |
| `slugify()` | `models.Slugify()` |
| `{% static 'file' %}` | `/static/file` — served by `http.FileServer` |
| `{% extends "base.html" %}` | `template.ParseFiles("base.html", "page.html")` |
| `{% block content %}` | `{{block "content" .}}` |
| `render(request, 'template.html', context)` | `render(w, r, data, "template.html")` |
| `@shared_task` (Celery) | `go notifyNewPost(...)` — a goroutine |
| Migrations | `db.MustExec(schema)` with `CREATE TABLE IF NOT EXISTS` |
| `mark_safe()` | `template.HTML(...)` |
| Custom template filter | `template.FuncMap{"markdown": ...}` |

The biggest takeaway: Django is a very well-designed set of opinions about
how to assemble the exact things you just built from scratch. Neither is
"better" — Django is faster to start, Go is faster to run and easier to
reason about at scale.