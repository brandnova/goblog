# GoBlog — Info

---

## Deployment on Leapcell

GoBlog is deployed on [Leapcell](https://leapcell.io) using a free PostgreSQL
database and a serverless web service. The steps below cover the full process
from database creation to a live URL.

---

### 1. Create the PostgreSQL Database

1. Sign up or log in at [leapcell.io](https://leapcell.io)
2. From your dashboard click **New Resource** → **PostgreSQL**
3. Give it a name — e.g. `goblog-db` — and confirm
4. Wait for it to provision (usually under a minute)
5. Click into the database resource and copy the **Connection String** — it
   looks like:
   ```
   postgres://username:password@host:5432/dbname
   ```
   Keep this somewhere safe — you will paste it as an environment variable
   in the next step.

The database schema (all five tables) is created automatically the first time
the app starts via `initDB()` in `db.go`. There is no separate migration step.

---

### 2. Create the Web Service

1. From your Leapcell dashboard click **New Service** → **Web Service**
2. Click **Connect GitHub**, authorise Leapcell, and select your `goblog` repo
3. Fill in the build settings:

| Field | Value |
|---|---|
| **Runtime** | Go |
| **Build Command** | `go build -o goblog .` |
| **Start Command** | `./goblog` |

---

### 3. Set Environment Variables

Before deploying, go to the **Environment** tab of your new service and add
the following two variables:

| Key | Value |
|---|---|
| `DATABASE_URL` | the connection string copied in Step 1 |
| `PORT` | `8080` |

---

### 4. Deploy

Click **Deploy**. Leapcell will pull your repo, run the build command, and
start the server. Watch the live build log — a successful deployment ends with:

```
Database ready.
Server running at http://localhost:8080
```

You will be given a free subdomain in the format:
```
https://goblog-xxxx.leapcell.app
```

From that point on, every `git push` to your `main` branch triggers an
automatic redeploy.

---

### Troubleshooting

**Build fails with missing module errors**
Run `go mod tidy` locally, commit the updated `go.mod` and `go.sum`, and push
again.

```bash
go mod tidy
git add go.mod go.sum
git commit -m "Tidy go modules"
git push
```

**`DATABASE_URL environment variable is not set`**
The environment variable name must match exactly. Check for extra spaces or
a typo in the Leapcell environment settings panel.

**Cover image uploads don't work in production**
Leapcell's serverless filesystem is read-only — file uploads to
`static/uploads/` will silently fail. Cover images work in local development.
Cloudflare R2 (free tier, no egress fees) is the recommended solution for
production image storage when you are ready to add it.

---

## Feature Ideas

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
    ID         int       `db:"id"`
    PostID     int       `db:"post_id"`
    UserID     int       `db:"user_id"`
    Body       string    `db:"body"`
    CreatedAt  time.Time `db:"created_at"`
    AuthorName string    `db:"author_name"`
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
func GetAllPublished(db *sqlx.DB, page, perPage int) ([]Post, error) {
    offset := (page - 1) * perPage
    db.Select(&posts, `... LIMIT $1 OFFSET $2`, perPage, offset)
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
shows posts only from people you follow — like Medium's following feed. This is
the feature that transforms the platform from "many blogs" to "a network."

**Full-text search with PostgreSQL**
Replace the current `ILIKE` search with PostgreSQL's native full-text search.
It supports ranking by relevance, stemming, and phrase queries — much smarter
than pattern matching.

```sql
-- Add a search vector column
ALTER TABLE posts ADD COLUMN search_vector TSVECTOR;

-- Query with ranking
SELECT *, ts_rank(search_vector, query) AS rank
FROM posts, plainto_tsquery('english', $1) query
WHERE search_vector @@ query
ORDER BY rank DESC;
```

**Image storage on Cloudflare R2**
Cover images currently save to `static/uploads/` on disk, which doesn't
persist on Leapcell's serverless plan. Cloudflare R2 is S3-compatible with
a generous free tier (10GB storage, no egress fees). The Go AWS SDK works
with R2 with a single endpoint change.

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
    Code    int
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
functions is straightforward — spin up a test PostgreSQL database, run your
queries against it, and assert the results.

```bash
go test ./...          # run all tests
go test -v ./models/   # verbose output for the models package
```

**`chi` router**
Once your route list grows, the stdlib mux starts to feel limited — no route
groups, no middleware per group. `chi` is the most Django-like Go router:
lightweight, composable, and fully compatible with `http.Handler`.

```go
r := chi.NewRouter()
r.Use(middleware.Logger)
r.Group(func(r chi.Router) {
    r.Use(RequireLogin)
    r.Get("/new", NewPostPage)
    r.Post("/new", CreatePost)
})
```

**`air` — live reload**
The Go equivalent of Django's auto-reloading dev server. Air watches your
files and restarts `go run .` automatically on every save.

```bash
go install github.com/air-verse/air@latest
air
```

---

## The Django → Go Mental Model, Fully Resolved

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
| `python-decouple` / `django-environ` | `godotenv` + `os.Getenv()` |