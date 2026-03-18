package main

import (
	"log"
	"net/http"
	"os"

	"goblog/handlers"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file in development.
	// godotenv.Load() silently does nothing if the file doesn't exist,
	// so this is safe to leave in production too.
	godotenv.Load()

	initDB()
	handlers.Init(db)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Public routes
	mux.HandleFunc("GET /{$}", handlers.Index)
	mux.HandleFunc("GET /u/{username}", handlers.Profile)
	mux.HandleFunc("GET /u/{username}/{slug}", handlers.PostDetail)
	mux.HandleFunc("GET /tag/{name}", handlers.PostsByTag)
	mux.HandleFunc("GET /search", handlers.SearchHandler)
	mux.HandleFunc("GET /register", handlers.RegisterPage)
	mux.HandleFunc("POST /register", handlers.Register)
	mux.HandleFunc("GET /login", handlers.LoginPage)
	mux.HandleFunc("POST /login", handlers.Login)
	mux.HandleFunc("POST /logout", handlers.Logout)

	// Protected routes
	mux.HandleFunc("GET /dashboard", handlers.RequireLogin(handlers.Dashboard))
	mux.HandleFunc("GET /bookmarks", handlers.RequireLogin(handlers.Bookmarks))
	mux.HandleFunc("GET /settings", handlers.RequireLogin(handlers.SettingsPage))
	mux.HandleFunc("POST /settings/profile", handlers.RequireLogin(handlers.UpdateProfile))
	mux.HandleFunc("POST /settings/password", handlers.RequireLogin(handlers.UpdatePassword))
	mux.HandleFunc("POST /post/{id}/bookmark", handlers.RequireLogin(handlers.BookmarkToggle))
	mux.HandleFunc("POST /post/{id}/react", handlers.RequireLogin(handlers.ReactionToggle))
	mux.HandleFunc("GET /new", handlers.RequireLogin(handlers.NewPostPage))
	mux.HandleFunc("POST /new", handlers.RequireLogin(handlers.CreatePost))
	mux.HandleFunc("GET /edit/{id}", handlers.RequireLogin(handlers.EditPostPage))
	mux.HandleFunc("POST /edit/{id}", handlers.RequireLogin(handlers.UpdatePost))
	mux.HandleFunc("POST /delete/{id}", handlers.RequireLogin(handlers.DeletePost))

	mux.HandleFunc("/", handlers.NotFound) // catch-all — must be last

	log.Printf("🚀 Server running at http://localhost:%s\n", port)
	// Middleware chain (outermost runs first):
	// CSRFMiddleware → AuthMiddleware → mux
	handler := handlers.CSRFMiddleware(handlers.AuthMiddleware(mux))
	log.Fatal(http.ListenAndServe(":"+port, handler))
}