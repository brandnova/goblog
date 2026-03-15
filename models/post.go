package models

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"
    "github.com/gomarkdown/markdown"
    "github.com/gomarkdown/markdown/html"
    "github.com/gomarkdown/markdown/parser"
)

// StripMarkdown removes common markdown syntax for plain text previews.
// Django parallel: the |striptags filter, but for markdown instead of HTML.
func StripMarkdown(body string) string {
    // First render to HTML, then strip the HTML tags
    rendered := RenderMarkdown(body)

    // Strip HTML tags by removing everything between < and >
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

    // Collapse excess whitespace left behind by removed tags
    stripped := strings.Join(strings.Fields(result.String()), " ")
    return stripped
}

// RenderMarkdown converts a markdown string to safe HTML.
// We call this from the template via a custom function — see helpers.go.
func RenderMarkdown(body string) string {
    extensions := parser.CommonExtensions | parser.AutoHeadingIDs
    p := parser.NewWithExtensions(extensions)

    htmlFlags := html.CommonFlags | html.HrefTargetBlank
    opts := html.RendererOptions{Flags: htmlFlags}
    renderer := html.NewRenderer(opts)

    return string(markdown.ToHTML([]byte(body), p, renderer))
}

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

    Tags       []Tag  `db:"-"`         // still "-" — sqlx can't scan into a slice
    AuthorName string `db:"author_name"` // ← fixed: now sqlx will map the alias
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

// Excerpt returns the first 200 characters of the body as a plain-text preview.
// Django parallel: a truncatechars template filter or a model property.
func (p *Post) Excerpt() string {
    // Strip markdown first so the preview is clean plain text
    plain := StripMarkdown(p.Body)
    if utf8.RuneCountInString(plain) <= 200 {
        return plain
    }
    runes := []rune(plain)
    return string(runes[:200]) + "…"
}

// Slugify converts a title string into a URL-friendly slug.
// e.g. "My Great Post!" → "my-great-post"
// Django parallel: django.utils.text.slugify()
func Slugify(title string) string {
	slug := strings.ToLower(title)
	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove characters that aren't alphanumeric or hyphens
	var b strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	// Collapse multiple hyphens into one
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
// Each function is explicit SQL, so you always know exactly what hits the DB.
// Django parallel: Model.objects.filter(), .get(), .create() etc.
// -----------------------------------------------------------------------

// GetAllPublished returns all published posts, newest first, with author name.
// Django parallel: Post.objects.filter(status='published').select_related('author')
func GetAllPublished(db *sqlx.DB) ([]Post, error) {
    var posts []Post
    err := db.Select(&posts, `
        SELECT
            p.id,
            p.user_id,
            p.title,
            p.slug,
            p.body,
            p.cover_image,
            p.status,
            p.created_at,
            p.updated_at,
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
        WHERE p.slug = ?
    `, slug)
    if err != nil {
        return nil, err
    }
    db.Select(&post.Tags, `
        SELECT t.* FROM tags t
        JOIN post_tags pt ON pt.tag_id = t.id
        WHERE pt.post_id = ?
    `, post.ID)
    return post, nil
}

func GetPostByID(db *sqlx.DB, id int) (*Post, error) {
    post := &Post{}
    err := db.Get(post, `
        SELECT
            p.id, p.user_id, p.title, p.slug, p.body,
            p.cover_image, p.status, p.created_at, p.updated_at,
            u.username AS author_name
        FROM posts p
        JOIN users u ON u.id = p.user_id
        WHERE p.id = ?
    `, id)
    if err != nil {
        return nil, err
    }
    db.Select(&post.Tags, `
        SELECT t.* FROM tags t
        JOIN post_tags pt ON pt.tag_id = t.id
        WHERE pt.post_id = ?
    `, post.ID)
    return post, nil
}

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
        WHERE t.name = ? AND p.status = 'published'
        ORDER BY p.created_at DESC
    `, tagName)
    return posts, err
}

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
          AND (p.title LIKE ? OR p.body LIKE ?)
        ORDER BY p.created_at DESC
    `, like, like)
    return posts, err
}

// CreatePost inserts a new post and syncs its tags.
// Django parallel: Post.objects.create(...) followed by post.tags.set(tags)
func CreatePost(db *sqlx.DB, userID int, title, body, status string, tags []string, coverImage string) error {
	slug := Slugify(title)
	result, err := db.Exec(`
		INSERT INTO posts (user_id, title, slug, body, status, cover_image)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, title, slug, body, status, coverImage)
	if err != nil {
		return err
	}

	postID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	return syncTags(db, int(postID), tags)
}

// UpdatePost updates an existing post's fields and re-syncs its tags.
// Django parallel: post.save() after modifying fields + post.tags.set(tags)
func UpdatePost(db *sqlx.DB, postID int, title, body, status string, tags []string, coverImage string) error {
	slug := Slugify(title)
	_, err := db.Exec(`
		UPDATE posts
		SET title = ?, slug = ?, body = ?, status = ?, cover_image = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, title, slug, body, status, coverImage, postID)
	if err != nil {
		return err
	}

	return syncTags(db, postID, tags)
}

// DeletePost removes a post. The ON DELETE CASCADE in the schema
// automatically deletes its post_tags rows too.
// Django parallel: post.delete()
func DeletePost(db *sqlx.DB, postID int) error {
	_, err := db.Exec("DELETE FROM posts WHERE id = ?", postID)
	return err
}

// GetPostsByUser returns ALL posts by a user regardless of status.
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
        WHERE p.user_id = ?
        ORDER BY p.created_at DESC
    `, userID)
    return posts, err
}

// -----------------------------------------------------------------------
// Private helpers
// -----------------------------------------------------------------------

// syncTags handles the ManyToMany relationship between posts and tags.
// It clears existing tags for the post, ensures each tag exists in the
// tags table, then links them via post_tags.
//
// Django does all of this automatically with post.tags.set([...]).
// Writing it manually shows you exactly what that ORM call does under the hood.
func syncTags(db *sqlx.DB, postID int, tagNames []string) error {
	// Remove all existing tag associations for this post
	if _, err := db.Exec("DELETE FROM post_tags WHERE post_id = ?", postID); err != nil {
		return err
	}

	for _, name := range tagNames {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}

		// INSERT OR IGNORE means: create the tag if it doesn't exist,
		// do nothing if it already does. No need to check first.
		// Django parallel: Tag.objects.get_or_create(name=name)
		if _, err := db.Exec("INSERT OR IGNORE INTO tags (name) VALUES (?)", name); err != nil {
			return err
		}

		var tag Tag
		if err := db.Get(&tag, "SELECT * FROM tags WHERE name = ?", name); err != nil {
			return err
		}

		if _, err := db.Exec(
			"INSERT INTO post_tags (post_id, tag_id) VALUES (?, ?)", postID, tag.ID,
		); err != nil {
			return err
		}
	}
	return nil
}