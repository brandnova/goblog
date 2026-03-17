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
See the Object Storage section below for the full solution.

---

## Object Storage for Cover Images

### Why local file storage doesn't work on Leapcell

When you run GoBlog locally, `handleCoverUpload()` in `handlers/posts.go` saves
uploaded images to `static/uploads/` on disk and stores the path in the
database. That works perfectly offline because the file stays on your filesystem
between requests.

On Leapcell's serverless plan, two things break this:

**The filesystem is read-only.** Only `/tmp` is writable, and even then only
for the lifetime of a single request. Writing to `static/uploads/` throws a
permission error silently — the handler returns `""` and no image is saved.

**The runtime is stateless and ephemeral.** Even if you wrote to `/tmp`, the
file would vanish the moment the instance goes dormant (after ~30 minutes of
inactivity). Every cold start starts with a clean slate.

This is not a Leapcell quirk — it is how all modern serverless and container
platforms work. The correct answer everywhere (Vercel, Render, Fly.io, Railway)
is to store uploaded files in a dedicated object storage service.

---

### What is object storage?

Object storage is a way of storing files (images, videos, documents, any binary
data) as self-contained "objects" in a flat namespace called a **bucket**, rather
than in a directory tree on a server's disk.

Each object gets a unique key (essentially a filename) and a publicly accessible
URL. You upload to the bucket via an API; the file is stored redundantly across
multiple servers by the provider; and you reference it in your app by its URL.

The dominant standard for object storage APIs is **Amazon S3** (Simple Storage
Service). Almost every cloud provider — including Leapcell — implements the same
S3 API, which means the Go code you write to talk to Leapcell object storage is
identical to code that talks to AWS S3, Cloudflare R2, or any other provider.
You only change the endpoint URL and credentials.

The flow for GoBlog cover images becomes:

```
User submits form with image
    → Go handler receives the file bytes
    → Handler uploads bytes to object storage bucket via S3 API
    → Object storage returns a public URL (e.g. https://my-bucket.leapcellobj.com/uploads/abc123.jpg)
    → Handler stores that URL in the database cover_image column
    → Template renders <img src="https://..."> — the browser fetches directly from object storage
```

The Go binary never serves the image itself. The browser fetches it directly
from the storage provider's CDN. This is faster, cheaper, and scales to any
number of users with zero extra work on the Go side.

---

### Setting up Leapcell Object Storage

Leapcell provides a free S3-compatible object storage service alongside its
web services and PostgreSQL.

**Step 1 — Create a bucket**

1. Log in to [leapcell.io](https://leapcell.io)
2. Dashboard → **New Resource** → **Object Storage**
3. Give the bucket a name — e.g. `goblog-images`
4. Once created, open the bucket details page
5. Note three values — you will need them as environment variables:
   - **Endpoint** — looks like `https://objstorage.leapcell.io`
   - **Access Key ID**
   - **Secret Access Key**
6. The public URL for your files will be:
   `https://goblog-images.leapcellobj.com` (bucket name + `.leapcellobj.com`)

**Step 2 — Add environment variables to your web service**

Go to your GoBlog web service → **Environment** tab and add:

| Key | Value |
|---|---|
| `S3_ENDPOINT` | `https://objstorage.leapcell.io` |
| `S3_ACCESS_KEY` | your Access Key ID |
| `S3_SECRET_KEY` | your Secret Access Key |
| `S3_BUCKET` | `goblog-images` |
| `S3_PUBLIC_URL` | `https://goblog-images.leapcellobj.com` |

These are also added to your local `.env` file for development — you can point
the local variables at the same Leapcell bucket, or set them to empty strings
to keep using local disk storage while developing.

---

### How to implement it in Go

The AWS SDK for Go v2 handles the S3 API. Install it:

```bash
go get github.com/aws/aws-sdk-go-v2/aws
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/aws/aws-sdk-go-v2/service/s3
```

**Create `storage/s3.go`** — a dedicated package for storage logic, keeping it
separate from handlers. This is the same separation Django achieves with
`django-storages` or a custom storage backend:

```go
package storage

import (
    "bytes"
    "context"
    "fmt"
    "mime"
    "os"
    "path/filepath"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/google/uuid"
)

var client *s3.Client
var bucket  string
var publicURL string

// Init sets up the S3 client using environment variables.
// Call this from main() alongside initDB().
// If S3_ENDPOINT is not set, Init is a no-op — local disk upload is used instead.
func Init() {
    endpoint  := os.Getenv("S3_ENDPOINT")
    accessKey := os.Getenv("S3_ACCESS_KEY")
    secretKey := os.Getenv("S3_SECRET_KEY")
    bucket     = os.Getenv("S3_BUCKET")
    publicURL  = os.Getenv("S3_PUBLIC_URL")

    if endpoint == "" {
        // Not configured — local disk mode, no S3 client needed
        return
    }

    cfg, err := config.LoadDefaultConfig(context.TODO(),
        config.WithCredentialsProvider(
            credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
        ),
        // "auto" works for all S3-compatible providers that aren't AWS
        config.WithRegion("auto"),
    )
    if err != nil {
        return
    }

    client = s3.NewFromConfig(cfg, func(o *s3.Options) {
        // Point the SDK at Leapcell (or any other S3-compatible endpoint)
        // instead of the default AWS endpoint
        o.BaseEndpoint = aws.String(endpoint)
        // PathStyle=true is required by most non-AWS S3-compatible services
        o.UsePathStyle = true
    })
}

// Enabled reports whether object storage is configured.
// Handlers call this to decide between S3 upload and local disk save.
func Enabled() bool {
    return client != nil
}

// Upload streams file bytes to the S3 bucket and returns the public URL.
// The key is a timestamped UUID — e.g. "uploads/1714000000-a1b2c3d4.jpg"
// This guarantees uniqueness and prevents filename collisions across users.
func Upload(data []byte, originalFilename string) (string, error) {
    ext := filepath.Ext(originalFilename)
    contentType := mime.TypeByExtension(ext)
    if contentType == "" {
        contentType = "application/octet-stream"
    }

    key := fmt.Sprintf("uploads/%d-%s%s", time.Now().Unix(), uuid.New().String()[:8], ext)

    _, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
        Bucket:      aws.String(bucket),
        Key:         aws.String(key),
        Body:        bytes.NewReader(data),
        ContentType: aws.String(contentType),
    })
    if err != nil {
        return "", err
    }

    return publicURL + "/" + key, nil
}
```

**Update `handleCoverUpload` in `handlers/posts.go`** to use S3 when available,
falling back to local disk when `storage.Enabled()` returns false:

```go
import "goblog/storage"

func handleCoverUpload(r *http.Request) string {
    file, header, err := r.FormFile("cover_image")
    if err != nil {
        return "" // no file uploaded — fine, it's optional
    }
    defer file.Close()

    // Read file bytes into memory — needed for both S3 and local save
    data, err := io.ReadAll(file)
    if err != nil {
        log.Println("Could not read uploaded file:", err)
        return ""
    }

    // ── S3 path (production) ─────────────────────────────────────────
    if storage.Enabled() {
        url, err := storage.Upload(data, header.Filename)
        if err != nil {
            log.Println("S3 upload failed:", err)
            return ""
        }
        return url
    }

    // ── Local disk path (development) ───────────────────────────────
    if err := os.MkdirAll("static/uploads", os.ModePerm); err != nil {
        log.Println("Could not create uploads directory:", err)
        return ""
    }
    filename := fmt.Sprintf("%d-%s", time.Now().Unix(), header.Filename)
    savePath  := "static/uploads/" + filename
    if err := os.WriteFile(savePath, data, 0644); err != nil {
        log.Println("Could not write file:", err)
        return ""
    }
    return "/static/uploads/" + filename
}
```

**Call `storage.Init()` in `main.go`**, right after `initDB()`:

```go
import "goblog/storage"

func main() {
    godotenv.Load()
    initDB()
    storage.Init() // add this line
    handlers.Init(db)
    // ... rest unchanged
}
```

---

### How the toggle works (and how to undo it)

The entire S3 integration is gated behind `storage.Enabled()`, which returns
`false` whenever the `S3_ENDPOINT` environment variable is not set. This means:

- **Locally with no S3 env vars set** → `storage.Enabled()` is `false` →
  `handleCoverUpload` saves to `static/uploads/` as before. Nothing changes
  for local development.

- **On Leapcell with S3 env vars set** → `storage.Enabled()` is `true` →
  uploads go to the bucket, images get public URLs, they load in the browser.

To revert to local-only storage at any time, simply remove the five S3
environment variables from the Leapcell environment panel. No code changes
needed. The app redeploys, `storage.Enabled()` returns false, and the local
disk path runs instead (though on Leapcell that means uploads silently fail,
since the disk is read-only — but the app continues to function normally for
posts without images).

---

### On your question about where Go saves files

Your impression — that the binary should just save files next to itself — is how
things work on a traditional VPS or dedicated server. If you rented a DigitalOcean
droplet and ran `./goblog` on it, `static/uploads/` would persist forever because
the disk is yours and it never gets wiped.

The serverless model trades that persistence for automatic scaling and zero idle
cost. The tradeoff is that any state the process writes to disk is ephemeral.
This is not unique to Leapcell — Heroku dynos, AWS Lambda, Google Cloud Run, and
Vercel all work the same way. Object storage is the universal solution the
industry settled on for this problem.

---

## Image Uploads — Leapcell Object Storage

### Why Cover Images Don't Work on Leapcell (Without This)

When you upload a cover image locally, Go saves it to `static/uploads/` on
disk and stores the path `/static/uploads/filename.jpg` in the database. This
works perfectly offline because the file persists next to the running binary.

On Leapcell's serverless plan the filesystem is **read-only** except for
`/tmp`. When the app tries to write to `static/uploads/`, the write silently
fails. The path gets saved to the database but the file was never written, so
the image URL returns a 404.

This is not a bug in the code — it is a fundamental property of serverless
platforms. Every deployment creates a fresh, immutable container from your
Git repo. Files written to disk at runtime disappear when the instance is
recycled. This is why serverless apps must externalise all file storage to a
dedicated service.

---

### Leapcell Object Storage (Beta)

Leapcell has its own S3-compatible object storage, currently in beta and
available free from the dashboard.

**How to create a bucket:**

1. Go to your Leapcell dashboard
2. Click **New Resource** → **Create Object Storage (+CDN) (Beta)**
3. Your bucket is provisioned immediately with a unique bucket name and
   CDN URL in the format:
   ```
   https://<subdomain>.leapcellobj.com/<bucket-name>/{fileKey}
   ```

**Free tier limits:**
- 100 MB total storage
- 5,000 files maximum
- CDN included — files are served from the closest edge node globally
- 12-hour CDN cache TTL
- Limits cannot be increased — this is a hard cap on the beta tier

**Important:** because this is beta, reliability is not guaranteed. For a
personal blog with small cover images this is perfectly fine. If you need
more storage later, Cloudflare R2 is the drop-in replacement (10 GB free,
zero egress fees) — the code is identical, only the endpoint and credentials
change.

---

### Connection Details

Leapcell Object Storage is S3-compatible. When you create a bucket, the
dashboard shows you:

- **Endpoint:** `https://objstorage.leapcell.io`
- **Bucket name:** your unique bucket name (e.g. `os-wsp...`)
- **Access Key ID:** shown in the dashboard
- **Secret Access Key:** shown in the dashboard
- **CDN base URL:** `https://<subdomain>.leapcellobj.com/<bucket-name>`

**⚠️ Never commit credentials to Git.** Store them as environment variables.

---

### Implementation

#### Step 1 — Install the AWS SDK

Leapcell Object Storage uses the S3 API, so the standard AWS SDK for Go
works without modification:

```bash
go get github.com/aws/aws-sdk-go-v2/aws
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/aws/aws-sdk-go-v2/service/s3
```

#### Step 2 — Add environment variables

Add these to your `.env` locally and to your Leapcell service's Environment
settings panel:

```
# Leapcell Object Storage
OBJ_ENDPOINT=https://objstorage.leapcell.io
OBJ_BUCKET=your-bucket-name
OBJ_ACCESS_KEY=your-access-key-id
OBJ_SECRET_KEY=your-secret-access-key
OBJ_CDN_URL=https://your-subdomain.leapcellobj.com/your-bucket-name
```

Leave these unset in local `.env` to keep using local disk storage during
development — the toggle logic handles it automatically.

#### Step 3 — Create `storage.go`

Create a new file `storage.go` in the project root. This is the only file
in the codebase that knows about object storage — everything else is
unchanged:

```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// UploadFile saves an uploaded file either to Leapcell Object Storage
// (production) or to local disk (development).
//
// The decision is made by checking OBJ_ENDPOINT — if it is not
// set, the app behaves exactly as it did before this feature was added.
// No other part of the codebase needs to change.
//
// Returns the public URL of the saved file, or "" on failure.
func UploadFile(file multipart.File, originalFilename string) string {
	// Prefix with timestamp to avoid name collisions — same as before
	filename := fmt.Sprintf("%d-%s", time.Now().Unix(), originalFilename)

	endpoint := os.Getenv("OBJ_ENDPOINT")
	if endpoint == "" {
		// Development: save to local disk
		return saveLocally(file, filename)
	}

	// Production: upload to Leapcell Object Storage
	return uploadToLeapcell(file, filename, endpoint)
}

// saveLocally writes the file to static/uploads/ — used in development.
func saveLocally(file multipart.File, filename string) string {
	if err := os.MkdirAll("static/uploads", os.ModePerm); err != nil {
		log.Println("Could not create uploads dir:", err)
		return ""
	}
	dst, err := os.Create("static/uploads/" + filename)
	if err != nil {
		log.Println("Could not create file:", err)
		return ""
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		log.Println("Could not write file:", err)
		return ""
	}
	return "/static/uploads/" + filename
}

// uploadToLeapcell uploads the file to Leapcell Object Storage via the
// S3-compatible API and returns the public CDN URL.
func uploadToLeapcell(file multipart.File, filename, endpoint string) string {
	bucket    := os.Getenv("OBJ_BUCKET")
	accessKey := os.Getenv("OBJ_ACCESS_KEY")
	secretKey := os.Getenv("OBJ_SECRET_KEY")
	cdnBase   := os.Getenv("OBJ_CDN_URL")

	// Read file into memory — needed for S3 PutObject
	data, err := io.ReadAll(file)
	if err != nil {
		log.Println("Could not read upload:", err)
		return ""
	}

	cfg := aws.Config{
		Region: "us-east-1", // required by SDK but ignored by Leapcell
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		),
		BaseEndpoint: aws.String(endpoint),
	}

	client := s3.NewFromConfig(cfg)

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		log.Println("Could not upload to object storage:", err)
		return ""
	}

	// Return the CDN URL — this is what gets stored in the database
	// and used in <img src="..."> tags in templates
	return cdnBase + "/" + filename
}
```

#### Step 4 — Update `handleCoverUpload` in `handlers/posts.go`

Replace the existing `handleCoverUpload` function with this version that
delegates to `UploadFile`:

```go
func handleCoverUpload(r *http.Request) string {
	file, header, err := r.FormFile("cover_image")
	if err != nil {
		return "" // no file uploaded — that is fine
	}
	defer file.Close()

	return UploadFile(file, header.Filename)
}
```

The `fmt`, `time`, and `os` imports that the old version used for local
saving have moved into `storage.go`, so remove them from `handlers/posts.go`
if they are no longer needed elsewhere in that file.

---

### How the Toggle Works

| `OBJ_ENDPOINT` set? | Where files go | URL stored in DB |
|---|---|---|
| No (local dev) | `static/uploads/` on disk | `/static/uploads/filename.jpg` |
| Yes (production) | Leapcell Object Storage | CDN URL from `OBJ_CDN_URL` |

To revert: remove `OBJ_ENDPOINT` from your Leapcell environment
variables and redeploy. The `storage.go` file can stay in the repo
permanently — without the env var it is completely inert.


To remove from your local project, remove the short `handleCoverUpload` function and uncomment the longer one. Then update the imports.

```go
import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"goblog/models"
)
```
---

### Fixing Posts With Broken Cover Images

If you created posts with cover images before adding object storage, those
posts have broken URLs in the database (the file was never actually saved to
disk on Leapcell). Fix them by editing each post in the dashboard and
re-uploading the cover image. The new upload will go to object storage and
the URL in the database updates automatically.

---

## Feature Ideas

### Tier 1 — Low Effort, High Impact

**Profile pages** — `/u/{username}` ✅ Done
Each user gets a public page listing all their published posts.

**Post slugs scoped to users** — `/u/{username}/{slug}` ✅ Done
Scoped URLs with UUID suffix guarantee uniqueness across all users and titles.

**Reading list / bookmarks** ✅ Done
A `bookmarks` join table with HTMX toggle — no page reload.

**Post reactions** (likes/claps) ✅ Done
A `reactions` table with HTMX toggle — count updates in place.

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

**Cover image uploads in production** — see Object Storage section above.

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
| `slugify()` | `models.Slugify()` + `UniqueSlug()` |
| `{% static 'file' %}` | `/static/file` — served by `http.FileServer` |
| `{% extends "base.html" %}` | `template.ParseFiles("base.html", "page.html")` |
| `{% block content %}` | `{{block "content" .}}` |
| `render(request, 'template.html', context)` | `render(w, r, data, "template.html")` |
| `@shared_task` (Celery) | `go notifyNewPost(...)` — a goroutine |
| Migrations | `db.MustExec(schema)` with `CREATE TABLE IF NOT EXISTS` |
| `mark_safe()` | `template.HTML(...)` |
| Custom template filter | `template.FuncMap{"markdown": ...}` |
| `python-decouple` / `django-environ` | `godotenv` + `os.Getenv()` |
| `django-storages` / custom storage backend | `storage/s3.go` with `storage.Enabled()` toggle |