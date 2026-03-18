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
	"github.com/google/uuid"
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
	Tags          []Tag  `db:"-"`
	AuthorName    string `db:"author_name"`
	ReactionCount int    `db:"reaction_count"`
	BookmarkCount int    `db:"bookmark_count"`
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

// UniqueSlug generates a slug with an 8-character UUID suffix to guarantee
// uniqueness across all users and posts.
// e.g. "My Great Post" → "my-great-post-a1b2c3d4"
// Django parallel: adding unique=True to SlugField and using uuid in pre_save
func UniqueSlug(title string) string {
	base := Slugify(title)
	suffix := uuid.New().String()[:8]
	return base + "-" + suffix
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
			u.username AS author_name,
			0 AS reaction_count, 0 AS bookmark_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.status = 'published'
		ORDER BY p.created_at DESC
	`)
	return posts, err
}

// GetPostByUserSlug fetches a single post by username + slug.
// Replaces GetPostBySlug now that slugs are scoped per user.
func GetPostByUserSlug(db *sqlx.DB, username, slug string) (*Post, error) {
	post := &Post{}
	err := db.Get(post, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			0 AS reaction_count, 0 AS bookmark_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE u.username = $1 AND p.slug = $2
	`, username, slug)
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

// GetPostByID fetches a post by its integer primary key.
// Used internally by the edit/delete handlers.
// Django parallel: Post.objects.get(pk=id)
func GetPostByID(db *sqlx.DB, id int) (*Post, error) {
	post := &Post{}
	err := db.Get(post, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			0 AS reaction_count, 0 AS bookmark_count
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

// GetPostsByUser returns ALL posts by a user regardless of status,
// including reaction and bookmark counts for the dashboard metrics row.
// Django parallel: Post.objects.filter(user=request.user).annotate(...)
func GetPostsByUser(db *sqlx.DB, userID int) ([]Post, error) {
	var posts []Post
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			(SELECT COUNT(*) FROM reactions r WHERE r.post_id = p.id) AS reaction_count,
			(SELECT COUNT(*) FROM bookmarks b WHERE b.post_id = p.id) AS bookmark_count
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
			u.username AS author_name,
			0 AS reaction_count, 0 AS bookmark_count
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
			u.username AS author_name,
			0 AS reaction_count, 0 AS bookmark_count
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
// Django parallel: Post.objects.create(...) followed by post.tags.set(tags)
func CreatePost(db *sqlx.DB, userID int, title, body, status string, tags []string, coverImage string) (string, error) {
	slug := UniqueSlug(title)

	var postID int
	err := db.QueryRow(`
		INSERT INTO posts (user_id, title, slug, body, status, cover_image)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, userID, title, slug, body, status, coverImage).Scan(&postID)
	if err != nil {
		return "", err
	}

	return slug, syncTags(db, postID, tags)
}

// UpdatePost updates a post's content fields and re-syncs its tags.
// The slug is intentionally NOT updated — it is permanent from creation.
// Django parallel: post.save() after modifying fields + post.tags.set(tags)
func UpdatePost(db *sqlx.DB, postID int, title, body, status string, tags []string, coverImage string) error {
	_, err := db.Exec(`
		UPDATE posts
		SET title = $1, body = $2, status = $3,
		    cover_image = $4, updated_at = NOW()
		WHERE id = $5
	`, title, body, status, coverImage, postID)
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
// Paginated query functions
// All paginated queries use LIMIT + OFFSET. Page is 1-indexed.
// -----------------------------------------------------------------------

const PerPage = 10

// GetAllPublishedPage returns one page of published posts with reaction counts.
func GetAllPublishedPage(db *sqlx.DB, page int) ([]Post, error) {
	var posts []Post
	offset := (page - 1) * PerPage
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			(SELECT COUNT(*) FROM reactions r WHERE r.post_id = p.id) AS reaction_count,
			0 AS bookmark_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.status = 'published'
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`, PerPage, offset)
	return posts, err
}

// CountPublished returns the total number of published posts.
func CountPublished(db *sqlx.DB) (int, error) {
	var n int
	err := db.Get(&n, "SELECT COUNT(*) FROM posts WHERE status = 'published'")
	return n, err
}

// GetPublishedByUserPage returns one page of a user's published posts.
func GetPublishedByUserPage(db *sqlx.DB, username string, page int) ([]Post, error) {
	var posts []Post
	offset := (page - 1) * PerPage
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			(SELECT COUNT(*) FROM reactions r WHERE r.post_id = p.id) AS reaction_count,
			0 AS bookmark_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE u.username = $1 AND p.status = 'published'
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
	`, username, PerPage, offset)
	return posts, err
}

// CountPublishedByUser returns the total published post count for a user.
func CountPublishedByUser(db *sqlx.DB, username string) (int, error) {
	var n int
	err := db.Get(&n, `
		SELECT COUNT(*) FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE u.username = $1 AND p.status = 'published'
	`, username)
	return n, err
}

// GetPostsByTagPage returns one page of posts for a tag.
func GetPostsByTagPage(db *sqlx.DB, tagName string, page int) ([]Post, error) {
	var posts []Post
	offset := (page - 1) * PerPage
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			0 AS reaction_count, 0 AS bookmark_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		JOIN post_tags pt ON pt.post_id = p.id
		JOIN tags t ON t.id = pt.tag_id
		WHERE t.name = $1 AND p.status = 'published'
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
	`, tagName, PerPage, offset)
	return posts, err
}

// CountPostsByTag returns the total post count for a tag.
func CountPostsByTag(db *sqlx.DB, tagName string) (int, error) {
	var n int
	err := db.Get(&n, `
		SELECT COUNT(*) FROM posts p
		JOIN post_tags pt ON pt.post_id = p.id
		JOIN tags t ON t.id = pt.tag_id
		WHERE t.name = $1 AND p.status = 'published'
	`, tagName)
	return n, err
}

// GetBookmarkedPostsPage returns one page of a user's bookmarked posts.
func GetBookmarkedPostsPage(db *sqlx.DB, userID int, page int) ([]Post, error) {
	var posts []Post
	offset := (page - 1) * PerPage
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			0 AS reaction_count, 0 AS bookmark_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		JOIN bookmarks b ON b.post_id = p.id
		WHERE b.user_id = $1 AND p.status = 'published'
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, PerPage, offset)
	return posts, err
}

// CountBookmarkedPosts returns the total bookmark count for a user.
func CountBookmarkedPosts(db *sqlx.DB, userID int) (int, error) {
	var n int
	err := db.Get(&n, `
		SELECT COUNT(*) FROM bookmarks b
		JOIN posts p ON p.id = b.post_id
		WHERE b.user_id = $1 AND p.status = 'published'
	`, userID)
	return n, err
}

// SearchPostsPage returns one page of search results with reaction counts.
func SearchPostsPage(db *sqlx.DB, query string, page int) ([]Post, error) {
	var posts []Post
	like := fmt.Sprintf("%%%s%%", query)
	offset := (page - 1) * PerPage
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			(SELECT COUNT(*) FROM reactions r WHERE r.post_id = p.id) AS reaction_count,
			0 AS bookmark_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE p.status = 'published'
		  AND (p.title ILIKE $1 OR p.body ILIKE $1)
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
	`, like, PerPage, offset)
	return posts, err
}

// CountSearchResults returns the total search result count.
func CountSearchResults(db *sqlx.DB, query string) (int, error) {
	var n int
	like := fmt.Sprintf("%%%s%%", query)
	err := db.Get(&n, `
		SELECT COUNT(*) FROM posts p
		WHERE p.status = 'published'
		  AND (p.title ILIKE $1 OR p.body ILIKE $1)
	`, like)
	return n, err
}

// -----------------------------------------------------------------------
// Tag attachment
// -----------------------------------------------------------------------

// AttachTags fetches tags for a slice of posts in a single query and
// attaches them to the correct post. Call this after any list query.
//
// Without this, tag pills never appear on list pages because the
// standard SELECT queries can't populate []Tag slices directly.
//
// Django parallel: prefetch_related('tags') on a queryset.
func AttachTags(db *sqlx.DB, posts []Post) error {
	if len(posts) == 0 {
		return nil
	}

	// Collect post IDs
	ids := make([]interface{}, len(posts))
	placeholders := ""
	for i, p := range posts {
		ids[i] = p.ID
		if i > 0 {
			placeholders += ","
		}
		placeholders += fmt.Sprintf("$%d", i+1)
	}

	// Single query fetches all tags for all posts in the slice
	rows, err := db.Query(fmt.Sprintf(`
		SELECT pt.post_id, t.id, t.name
		FROM tags t
		JOIN post_tags pt ON pt.tag_id = t.id
		WHERE pt.post_id IN (%s)
		ORDER BY pt.post_id, t.name
	`, placeholders), ids...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Build a map of postID → []Tag
	tagMap := make(map[int][]Tag)
	for rows.Next() {
		var postID int
		var tag Tag
		if err := rows.Scan(&postID, &tag.ID, &tag.Name); err != nil {
			continue
		}
		tagMap[postID] = append(tagMap[postID], tag)
	}

	// Attach to the posts slice by reference
	for i := range posts {
		posts[i].Tags = tagMap[posts[i].ID]
	}

	return nil
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

// GetPublishedByUser returns published posts for a given username.
// Used on public profile pages — drafts are never shown.
// Django parallel: Post.objects.filter(user__username=username, status='published')
func GetPublishedByUser(db *sqlx.DB, username string) ([]Post, error) {
	var posts []Post
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			(SELECT COUNT(*) FROM reactions r WHERE r.post_id = p.id) AS reaction_count,
			0 AS bookmark_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		WHERE u.username = $1 AND p.status = 'published'
		ORDER BY p.created_at DESC
	`, username)
	return posts, err
}

// ToggleBookmark adds or removes a bookmark. Returns true if now bookmarked.
// Django parallel: bookmark, created = Bookmark.objects.get_or_create(...)
//                  if not created: bookmark.delete()
func ToggleBookmark(db *sqlx.DB, userID, postID int) (bool, error) {
	var exists bool
	db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM bookmarks WHERE user_id=$1 AND post_id=$2)", userID, postID)

	if exists {
		_, err := db.Exec("DELETE FROM bookmarks WHERE user_id=$1 AND post_id=$2", userID, postID)
		return false, err
	}

	_, err := db.Exec("INSERT INTO bookmarks (user_id, post_id) VALUES ($1, $2)", userID, postID)
	return true, err
}

// IsBookmarked checks if a user has bookmarked a post.
func IsBookmarked(db *sqlx.DB, userID, postID int) bool {
	var exists bool
	db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM bookmarks WHERE user_id=$1 AND post_id=$2)", userID, postID)
	return exists
}

// GetBookmarkedPosts returns all posts bookmarked by a user.
func GetBookmarkedPosts(db *sqlx.DB, userID int) ([]Post, error) {
	var posts []Post
	err := db.Select(&posts, `
		SELECT
			p.id, p.user_id, p.title, p.slug, p.body,
			p.cover_image, p.status, p.created_at, p.updated_at,
			u.username AS author_name,
			0 AS reaction_count, 0 AS bookmark_count
		FROM posts p
		JOIN users u ON u.id = p.user_id
		JOIN bookmarks b ON b.post_id = p.id
		WHERE b.user_id = $1 AND p.status = 'published'
		ORDER BY b.created_at DESC
	`, userID)
	return posts, err
}

// ToggleReaction adds or removes a reaction. Returns the new total count.
// Django parallel: Like.objects.get_or_create(...) / like.delete()
func ToggleReaction(db *sqlx.DB, userID, postID int) (int, error) {
	var exists bool
	db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM reactions WHERE user_id=$1 AND post_id=$2)", userID, postID)

	if exists {
		db.Exec("DELETE FROM reactions WHERE user_id=$1 AND post_id=$2", userID, postID)
	} else {
		db.Exec("INSERT INTO reactions (user_id, post_id) VALUES ($1, $2)", userID, postID)
	}

	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM reactions WHERE post_id=$1", postID)
	return count, err
}

// GetReactionCount returns the total reaction count for a post.
func GetReactionCount(db *sqlx.DB, postID int) int {
	var count int
	db.Get(&count, "SELECT COUNT(*) FROM reactions WHERE post_id=$1", postID)
	return count
}

// HasReacted checks if a user has reacted to a post.
func HasReacted(db *sqlx.DB, userID, postID int) bool {
	var exists bool
	db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM reactions WHERE user_id=$1 AND post_id=$2)", userID, postID)
	return exists
}