package models

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/jmoiron/sqlx"
)

// Post maps to the posts table.
// Fields marked `db:"-"` are NOT database columns — we populate them
// manually after the main query (e.g. tags, author name).
// Django parallel: a Model class. The `db:"-"` fields are like
// @property methods or annotated fields.
type Post struct {
	ID         int       `db:"id"`
	UserID     int       `db:"user_id"`
	Title      string    `db:"title"`
	Slug       string    `db:"slug"`
	Body       string    `db:"body"`
	CoverImage string    `db:"cover_image"`
	Status     string    `db:"status"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`

	// Populated manually via JOIN — not real columns on the posts table.
	// db:"author_name" must match the AS alias in every query that fetches posts.
	Tags       []Tag  `db:"-"`
	AuthorName string `db:"author_name"`
}

// Tag maps to the tags table.
type Tag struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

// ReadTime estimates reading time in minutes based on ~200 words per minute.
// Because this is a method on *Post, you can call {{ .Data.ReadTime }}
// directly in your templates — just like a Django model method.
func (p *Post) ReadTime() int {
	wordCount := utf8.RuneCountInString(p.Body)
	minutes := math.Ceil(float64(wordCount) / 200.0)
	if minutes < 1 {
		return 1
	}
	return int(minutes)
}

// Excerpt returns a plain-text preview stripped of markdown syntax.
// Django parallel: a truncatechars + striptags template filter combo.
func (p *Post) Excerpt() string {
	plain := StripMarkdown(p.Body)
	if utf8.RuneCountInString(plain) <= 200 {
		return plain
	}
	runes := []rune(plain)
	return string(runes[:200]) + "…"
}

// RenderMarkdown converts a markdown string to safe HTML.
// Called from the template FuncMap registered in handlers/helpers.go.
// Django parallel: a custom template filter using bleach or markdown2.
func RenderMarkdown(body string) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return string(markdown.ToHTML([]byte(body), p, renderer))
}

// StripMarkdown renders markdown to HTML then strips all HTML tags,
// returning clean plain text suitable for excerpts and search results.
// Django parallel: the |striptags template filter.
func StripMarkdown(body string) string {
	rendered := RenderMarkdown(body)

	var result strings.Builder
	inTag := false
	for _, r := range rendered {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// Collapse whitespace left behind by removed tags
	return strings.Join(strings.Fields(result.String()), " ")
}

// Slugify converts a title string into a URL-friendly slug.
// e.g. "My Great Post!" → "my-great-post"
// Django parallel: django.utils.text.slugify()
func Slugify(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	var b strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

// TagsToString converts a []Tag slice back to a comma-separated string.
// Used to pre-fill the tags input on the edit form.
// e.g. [{1, "go"}, {2, "tutorial"}] → "go, tutorial"
func TagsToString(tags []Tag) string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}

// -----------------------------------------------------------------------
// Query functions — your "ORM layer"
// All placeholders use $1, $2, $3 — PostgreSQL syntax.
// Django parallel: Model.objects.filter(), .get(), .create() etc.
// -----------------------------------------------------------------------

// GetAllPublished returns all published posts, newest first, with author name.
// Django parallel: Post.objects.filter(status='published').select_related('author')
func GetAllPublished(db *sqlx.DB) ([]Post, error) {
	var posts []Post
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.status = 'published'
		ORDER BY p.created_at DESC
	`)
	return posts, err
}

// GetPostBySlug fetches a single post by its slug, including tags.
// Django parallel: get_object_or_404(Post, slug=slug)
func GetPostBySlug(db *sqlx.DB, slug string) (*Post, error) {
	post := &Post{}
	err := db.Get(post, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.slug = $1
	`, slug)
	if err != nil {
		return nil, err
	}

	// Fetch associated tags in a second query.
	// Django does this automatically via ManyToManyField + prefetch_related.
	db.Select(&post.Tags, `
		SELECT t.* FROM tags t
		JOIN post_tags pt ON pt.tag_id = t.id
		WHERE pt.post_id = $1
	`, post.ID)

	return post, nil
}

// GetPostByID fetches a post by its integer primary key.
// Used internally by the edit/delete handlers.
// Django parallel: Post.objects.get(pk=id)
func GetPostByID(db *sqlx.DB, id int) (*Post, error) {
	post := &Post{}
	err := db.Get(post, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.id = $1
	`, id)
	if err != nil {
		return nil, err
	}

	db.Select(&post.Tags, `
		SELECT t.* FROM tags t
		JOIN post_tags pt ON pt.tag_id = t.id
		WHERE pt.post_id = $1
	`, post.ID)

	return post, nil
}

// GetPostsByUser returns ALL posts by a user regardless of status.
// Used by the dashboard to show drafts and published posts together.
// Django parallel: Post.objects.filter(user=request.user)
func GetPostsByUser(db *sqlx.DB, userID int) ([]Post, error) {
	var posts []Post
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.user_id = $1
		ORDER BY p.created_at DESC
	`, userID)
	return posts, err
}

// GetPostsByTag returns all published posts for a given tag name.
// Django parallel: Post.objects.filter(tags__name=tagName, status='published')
func GetPostsByTag(db *sqlx.DB, tagName string) ([]Post, error) {
	var posts []Post
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name
		FROM posts p
		JOIN users u ON u.id = p.user_id
		JOIN post_tags pt ON pt.post_id = p.id
		JOIN tags t ON t.id = pt.tag_id
		WHERE t.name = $1 AND p.status = 'published'
		ORDER BY p.created_at DESC
	`, tagName)
	return posts, err
}

// SearchPosts performs a case-insensitive full-text search on title and body.
// ILIKE is PostgreSQL's case-insensitive LIKE — replaces SQLite's LIKE.
// Django parallel: Post.objects.filter(Q(title__icontains=q) | Q(body__icontains=q))
func SearchPosts(db *sqlx.DB, query string) ([]Post, error) {
	var posts []Post
	like := fmt.Sprintf("%%%s%%", query)
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.status = 'published'
		  AND (p.title ILIKE $1 OR p.body ILIKE $1)
		ORDER BY p.created_at DESC
	`, like)
	return posts, err
}

// CreatePost inserts a new post and syncs its tags.
// RETURNING id is PostgreSQL's way of getting the new row's ID in one query.
// In SQLite we used result.LastInsertId() — this is cleaner.
// Django parallel: Post.objects.create(...) followed by post.tags.set(tags)
func CreatePost(db *sqlx.DB, userID int, title, body, status string, tags []string, coverImage string) error {
	slug := Slugify(title)

	var postID int
	err := db.QueryRow(`
		INSERT INTO posts (user_id, title, slug, body, status, cover_image)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, userID, title, slug, body, status, coverImage).Scan(&postID)
	if err != nil {
		return err
	}

	return syncTags(db, postID, tags)
}

// UpdatePost updates an existing post's fields and re-syncs its tags.
// Django parallel: post.save() after modifying fields + post.tags.set(tags)
func UpdatePost(db *sqlx.DB, postID int, title, body, status string, tags []string, coverImage string) error {
	slug := Slugify(title)
	_, err := db.Exec(`
		UPDATE posts
		SET title = $1, slug = $2, body = $3, status = $4,
		    cover_image = $5, updated_at = NOW()
		WHERE id = $6
	`, title, slug, body, status, coverImage, postID)
	if err != nil {
		return err
	}

	return syncTags(db, postID, tags)
}

// DeletePost removes a post. The ON DELETE CASCADE on post_tags means
// the tag associations are deleted automatically.
// Django parallel: post.delete()
func DeletePost(db *sqlx.DB, postID int) error {
	_, err := db.Exec("DELETE FROM posts WHERE id = $1", postID)
	return err
}

// -----------------------------------------------------------------------
// Private helpers
// -----------------------------------------------------------------------

// syncTags handles the ManyToMany relationship between posts and tags.
// ON CONFLICT DO NOTHING is PostgreSQL's equivalent of SQLite's INSERT OR IGNORE.
// Django parallel: post.tags.set([...])
func syncTags(db *sqlx.DB, postID int, tagNames []string) error {
	if _, err := db.Exec("DELETE FROM post_tags WHERE post_id = $1", postID); err != nil {
		return err
	}

	for _, name := range tagNames {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}

		// Create the tag if it doesn't exist, do nothing if it does.
		// Django parallel: Tag.objects.get_or_create(name=name)
		if _, err := db.Exec(
			"INSERT INTO tags (name) VALUES ($1) ON CONFLICT DO NOTHING", name,
		); err != nil {
			return err
		}

		var tag Tag
		if err := db.Get(&tag, "SELECT * FROM tags WHERE name = $1", name); err != nil {
			return err
		}

		if _, err := db.Exec(
			"INSERT INTO post_tags (post_id, tag_id) VALUES ($1, $2)", postID, tag.ID,
		); err != nil {
			return err
		}
	}
	return nil
}