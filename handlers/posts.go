package handlers

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

// -----------------------------------------------------------------------
// Public handlers — no login required
// -----------------------------------------------------------------------

// Index lists all published posts (GET /)
// Django parallel: a ListView with queryset = Post.objects.filter(status='published')
func Index(w http.ResponseWriter, r *http.Request) {
	posts, err := models.GetAllPublished(DB)
	if err != nil {
		http.Error(w, "Could not fetch posts: "+err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, posts, "templates/index.html")
}

// PostDetail shows a single post. (GET /u/{username}/{slug})
func PostDetail(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	slug := r.PathValue("slug")

	post, err := models.GetPostByUserSlug(DB, username, slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	isBookmarked := false
	hasReacted := false
	if u := CurrentUser(r); u != nil {
		isBookmarked = models.IsBookmarked(DB, u.ID, post.ID)
		hasReacted = models.HasReacted(DB, u.ID, post.ID)
	}

	render(w, r, map[string]any{
		"Post":        post,
		"Bookmarked":  isBookmarked,
		"HasReacted":  hasReacted,
		"ReactionCount": models.GetReactionCount(DB, post.ID),
	}, "templates/post.html")
}

// PostsByTag lists all published posts for a given tag (GET /tag/{name})
// Django parallel: a filtered ListView
func PostsByTag(w http.ResponseWriter, r *http.Request) {
	tagName := r.PathValue("name")

	posts, err := models.GetPostsByTag(DB, tagName)
	if err != nil {
		http.Error(w, "Could not fetch posts", http.StatusInternalServerError)
		return
	}

	// We pass both the posts and the tag name so the template can
	// render a heading like "Posts tagged: go"
	render(w, r, map[string]any{
		"Posts":   posts,
		"TagName": tagName,
	}, "templates/tag.html")
}

// SearchHandler handles HTMX live search (GET /search?q=...)
//
// This is an HTMX partial endpoint — it returns an HTML fragment,
// not a full page. HTMX calls this on every keystroke (debounced)
// and swaps the result into #search-results on the index page.
//
// Django parallel: a view that returns an HttpResponse with a
// rendered {% include 'partial.html' %} snippet.
func SearchHandler(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	if query == "" {
		// Return empty response — HTMX will clear #search-results
		w.Write([]byte(""))
		return
	}

	posts, err := models.SearchPosts(DB, query)
	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Render only the partial template — no base.html
	renderPartial(w, posts, "templates/partials/search_results.html")
}

// -----------------------------------------------------------------------
// Protected handlers — RequireLogin() is applied in main.go
// -----------------------------------------------------------------------

// NewPostPage renders the blank post creation form (GET /new)
func NewPostPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, nil, "templates/form.html")
}

// CreatePost handles the post creation form submission (POST /new)
// Django parallel: CreateView.form_valid()
func CreatePost(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form to support file uploads (cover image)
	// 10 << 20 = 10MB max — like Django's FILE_UPLOAD_MAX_MEMORY_SIZE
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		// Fall back to regular form parsing if no file was uploaded
		r.ParseForm()
	}

	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	status := r.FormValue("status")   // "draft" or "published"
	tagsInput := r.FormValue("tags")  // comma-separated: "go, tutorial, web"

	// Basic validation
	if title == "" || body == "" {
		render(w, r, map[string]any{
			"Error": "Title and body are required.",
			"Title": title,
			"Body":  body,
		}, "templates/form.html")
		return
	}

	if status != "draft" && status != "published" {
		status = "draft"
	}

	// Parse tags from comma-separated string into a slice
	// Django parallel: form.cleaned_data['tags']
	tags := parseTags(tagsInput)

	// Get the logged-in user from context
	// Django parallel: request.user
	user := CurrentUser(r)

	// Handle optional cover image upload
	coverImagePath := handleCoverUpload(r)

	err := models.CreatePost(DB, user.ID, title, body, status, tags, coverImagePath)
	if err != nil {
		render(w, r, map[string]any{
			"Error": "Could not save post: " + err.Error(),
			"Title": title,
			"Body":  body,
		}, "templates/form.html")
		return
	}

	// Background notification — the 'go' keyword runs this concurrently.
	// The HTTP redirect happens IMMEDIATELY; notifyNewPost runs on its own.
	// Django parallel: calling a Celery task with .delay()
	go notifyNewPost(title, user.Username)

	http.Redirect(w, r, "/u/"+user.Username+"/"+models.Slugify(title), http.StatusSeeOther)
}

// EditPostPage renders the edit form pre-filled with existing post data (GET /edit/{id})
// Django parallel: UpdateView — it fetches the object and pre-populates the form
func EditPostPage(w http.ResponseWriter, r *http.Request) {
	post, ok := getPostForEdit(w, r)
	if !ok {
		return // getPostForEdit already wrote the error response
	}

	render(w, r, map[string]any{
		"Post": post,
		"Tags": models.TagsToString(post.Tags), // convert []Tag back to "go, tutorial"
	}, "templates/form.html")
}

// UpdatePost handles the edit form submission (POST /edit/{id})
// Django parallel: UpdateView.form_valid()
func UpdatePost(w http.ResponseWriter, r *http.Request) {
	post, ok := getPostForEdit(w, r)
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		r.ParseForm()
	}

	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	status := r.FormValue("status")
	tagsInput := r.FormValue("tags")

	if title == "" || body == "" {
		render(w, r, map[string]any{
			"Error": "Title and body are required.",
			"Post":  post,
			"Tags":  tagsInput,
		}, "templates/form.html")
		return
	}

	if status != "draft" && status != "published" {
		status = post.Status // keep existing status if invalid value sent
	}

	tags := parseTags(tagsInput)

	// Only update cover image if a new file was actually uploaded
	coverImagePath := handleCoverUpload(r)
	if coverImagePath == "" {
		coverImagePath = post.CoverImage // keep existing cover
	}

	err := models.UpdatePost(DB, post.ID, title, body, status, tags, coverImagePath)
	if err != nil {
		render(w, r, map[string]any{
			"Error": "Could not update post: " + err.Error(),
			"Post":  post,
		}, "templates/form.html")
		return
	}

	// Redirect to the updated post — slug may have changed if title changed
	user := CurrentUser(r)
	newSlug := models.Slugify(title)
	http.Redirect(w, r, "/u/"+user.Username+"/"+newSlug, http.StatusSeeOther)
}

// DeletePost deletes a post by ID (POST /delete/{id})
// We use POST not DELETE because HTML forms only support GET and POST.
// Django parallel: DeleteView
func DeletePost(w http.ResponseWriter, r *http.Request) {
	post, ok := getPostForEdit(w, r)
	if !ok {
		return
	}

	if err := models.DeletePost(DB, post.ID); err != nil {
		http.Error(w, "Could not delete post", http.StatusInternalServerError)
		return
	}

	// If this was an HTMX request, return a 200 with empty body —
	// HTMX will remove the element from the DOM automatically.
	// Otherwise redirect to home as normal.
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Dashboard shows the logged-in user's posts — drafts and published.
// GET /dashboard
func Dashboard(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	posts, err := models.GetPostsByUser(DB, user.ID)
	if err != nil {
		http.Error(w, "Could not fetch your posts: "+err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, posts, "templates/dashboard.html")
}

// -----------------------------------------------------------------------
// Private helper functions — used only within this file
// -----------------------------------------------------------------------

// getPostForEdit fetches a post by ID from the URL and verifies that the
// logged-in user is the author. Returns false if it wrote an error response.
//
// This is used by EditPostPage, UpdatePost, and DeletePost to avoid
// repeating the same fetch-and-ownership-check logic.
//
// Django parallel: get_object_or_404() + UserPassesTestMixin
func getPostForEdit(w http.ResponseWriter, r *http.Request) (*models.Post, bool) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}

	post, err := models.GetPostByID(DB, id)
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}

	// Ownership check — only the author can edit or delete their post
	user := CurrentUser(r)
	if post.UserID != user.ID {
		http.Error(w, "You are not allowed to edit this post.", http.StatusForbidden)
		return nil, false
	}

	return post, true
}

// handleCoverUpload processes the cover_image field from a multipart form.
// Returns the public path to the saved file, or "" if no file was uploaded.
//
// Note: on Leapcell's serverless plan the filesystem is read-only except
// for /tmp. Cover image uploads will silently return "" in that environment.
// For production file storage, use an object store like Cloudflare R2.
func handleCoverUpload(r *http.Request) string {
	file, header, err := r.FormFile("cover_image")
	if err != nil {
		// No file uploaded — that's fine, cover image is optional
		return ""
	}
	defer file.Close()

	// Make sure the uploads directory exists
	if err := os.MkdirAll("static/uploads", os.ModePerm); err != nil {
		log.Println("Could not create uploads directory:", err)
		return ""
	}

	// Prefix the filename with a Unix timestamp to avoid name collisions
	// e.g. "1714000000-my-photo.jpg"
	filename := fmt.Sprintf("%d-%s", time.Now().Unix(), header.Filename)
	savePath := "static/uploads/" + filename

	dst, err := os.Create(savePath)
	if err != nil {
		log.Println("Could not save uploaded file:", err)
		return ""
	}
	defer dst.Close()

	// io.Copy streams the upload to disk without loading it all into memory
	// Django parallel: default_storage.save()
	if _, err := io.Copy(dst, file); err != nil {
		log.Println("Could not write uploaded file:", err)
		return ""
	}

	// Return the URL path (not the filesystem path) so we can store it in the DB
	return "/static/uploads/" + filename
}

// parseTags converts a comma-separated tag string into a cleaned slice.
// e.g. "Go,  tutorial , web" → ["go", "tutorial", "web"]
func parseTags(input string) []string {
	raw := strings.Split(input, ",")
	tags := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// notifyNewPost is a background task launched with 'go notifyNewPost(...)'
// It simulates sending a notification (email, Slack ping, etc.) without
// blocking the HTTP response.
//
// Django parallel: a Celery task decorated with @shared_task
func notifyNewPost(title, author string) {
	// In a real app, you'd send an email or ping a webhook here.
	// The time.Sleep simulates the delay of an external API call.
	time.Sleep(1 * time.Second)
	log.Printf("📨  New post published: \"%s\" by %s\n", title, author)
}

// Profile shows a user's public page with all their published posts.
// GET /u/{username}
// Django parallel: a DetailView on User with related published posts
func Profile(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	// Fetch the user so we can show their info even if they have no posts
	profileUser, err := models.GetUserByUsername(DB, username)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	posts, err := models.GetPublishedByUser(DB, username)
	if err != nil {
		http.Error(w, "Could not fetch posts", http.StatusInternalServerError)
		return
	}

	render(w, r, map[string]any{
		"ProfileUser": profileUser,
		"Posts":       posts,
	}, "templates/profile.html")
}

// BookmarkToggle handles HTMX bookmark toggle (POST /post/{id}/bookmark)
// Returns an HTML fragment — just the updated button — which HTMX swaps in.
func BookmarkToggle(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	bookmarked, err := models.ToggleBookmark(DB, user.ID, id)
	if err != nil {
		http.Error(w, "Could not update bookmark", http.StatusInternalServerError)
		return
	}

	// Return just the button fragment — HTMX swaps it into #bookmark-btn
	label := "Bookmark"
	style := "btn-ghost"
	if bookmarked {
		label = "Bookmarked ✓"
		style = "btn-primary"
	}

	fmt.Fprintf(w, `<button
		id="bookmark-btn"
		class="%s"
		hx-post="/post/%d/bookmark"
		hx-target="#bookmark-btn"
		hx-swap="outerHTML">
		%s
	</button>`, style, id, label)
}

// Bookmarks shows the logged-in user's saved posts. (GET /bookmarks)
func Bookmarks(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	posts, err := models.GetBookmarkedPosts(DB, user.ID)
	if err != nil {
		http.Error(w, "Could not fetch bookmarks", http.StatusInternalServerError)
		return
	}
	render(w, r, posts, "templates/bookmarks.html")
}

// ReactionToggle handles HTMX reaction toggle (POST /post/{id}/react)
// Returns just the updated reaction button fragment.
func ReactionToggle(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	count, err := models.ToggleReaction(DB, user.ID, id)
	if err != nil {
		http.Error(w, "Could not update reaction", http.StatusInternalServerError)
		return
	}

	reacted := models.HasReacted(DB, user.ID, id)
	style := "btn-ghost"
	if reacted {
		style = "btn-primary"
	}

	fmt.Fprintf(w, `<button
		id="reaction-btn"
		class="%s"
		style="display:inline-flex; align-items:center; gap:0.5rem;"
		hx-post="/post/%d/react"
		hx-target="#reaction-btn"
		hx-swap="outerHTML">
		♥ <span>%d</span>
	</button>`, style, id, count)
}