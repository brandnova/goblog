package main

import (
	"log"
	"net/http"

	"goblog/handlers"
)

func main() {
	// Initialize the database and run migrations
	initDB()

	// Give the handlers package access to the DB connection.
	// We do this because 'db' lives in main (db.go), and handlers
	// is a separate package — packages can't see each other's variables
	// unless explicitly passed. Django does this transparently via settings.py.
	handlers.Init(db)

	mux := http.NewServeMux()

	// ---------------------------------------------------------------
	// Static files
	// Go equivalent of Django's STATICFILES_DIRS + {% static %} tag
	// ---------------------------------------------------------------
	mux.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("./static"))))

	// ---------------------------------------------------------------
	// Public routes — no login required
	// Django parallel: urlpatterns in urls.py
	// ---------------------------------------------------------------
	mux.HandleFunc("GET /{$}", handlers.Index)           // exact root only
	mux.HandleFunc("GET /post/{slug}", handlers.PostDetail)
	mux.HandleFunc("GET /tag/{name}", handlers.PostsByTag)
	mux.HandleFunc("GET /search", handlers.SearchHandler)

	// Auth routes
	mux.HandleFunc("GET /register", handlers.RegisterPage)
	mux.HandleFunc("POST /register", handlers.Register)
	mux.HandleFunc("GET /login", handlers.LoginPage)
	mux.HandleFunc("POST /login", handlers.Login)
	mux.HandleFunc("POST /logout", handlers.Logout)

	// ---------------------------------------------------------------
	// Protected routes — wrapped with RequireLogin
	// Django parallel: @login_required or LoginRequiredMixin
	// ---------------------------------------------------------------
	mux.HandleFunc("GET /new", handlers.RequireLogin(handlers.NewPostPage))
	mux.HandleFunc("POST /new", handlers.RequireLogin(handlers.CreatePost))
	mux.HandleFunc("GET /edit/{id}", handlers.RequireLogin(handlers.EditPostPage))
	mux.HandleFunc("POST /edit/{id}", handlers.RequireLogin(handlers.UpdatePost))
	mux.HandleFunc("POST /delete/{id}", handlers.RequireLogin(handlers.DeletePost))
    mux.HandleFunc("GET /dashboard", handlers.RequireLogin(handlers.Dashboard))

	// ---------------------------------------------------------------
	// Wrap the entire router with AuthMiddleware.
	// This runs on EVERY request — it reads the session cookie, looks
	// up the user, and attaches them to the request context.
	//
	// Django parallel: adding SessionMiddleware + AuthenticationMiddleware
	// to the MIDDLEWARE list in settings.py.
	// ---------------------------------------------------------------
	log.Println("🚀 Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", handlers.AuthMiddleware(mux)))
}