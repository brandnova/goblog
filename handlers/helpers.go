package handlers

import (
    "html/template"
    "net/http"
    "goblog/models"
    "github.com/jmoiron/sqlx"
)

// DB is the shared database connection.
// It's set once in main.go via handlers.Init(db)
// This is the Go equivalent of Django's db connection being globally available
var DB *sqlx.DB

// Init is called from main.go to give the handlers package access to the DB.
// Think of it like Django's AppConfig.ready() — a setup step called at startup.
func Init(db *sqlx.DB) {
	DB = db
}

// Template function map — registers custom functions available in all templates.
// Django parallel: registering a custom template filter with @register.filter
var templateFuncs = template.FuncMap{
    // "markdown" is now callable in any template as {{ markdown .Body }}
    "markdown": func(body string) template.HTML {
        // template.HTML tells Go's template engine: "trust this HTML, don't escape it"
        // Without this, the rendered HTML tags would show as literal text — &lt;p&gt; etc.
        // Django's equivalent is marking a string as |safe or using mark_safe()
        return template.HTML(models.RenderMarkdown(body))
    },
}

// render parses base.html + the given page template and executes base.html
// as the entry point. This is our reusable Django-style render() shortcut.
//
// Usage:
//
//	render(w, r, someData, "templates/index.html")

func render(w http.ResponseWriter, r *http.Request, data any, pageTemplate string) {
	// We always inject the logged-in user into template data so every
	// template can access {{ .User }} — just like Django's request.user
    type TemplateData struct {
        Data any
        User any
    }

    td := TemplateData{
        Data: data,
        User: CurrentUser(r),
    }

    // Pass the FuncMap when parsing — this registers "markdown" for use in templates
    tmpl, err := template.New("").Funcs(templateFuncs).ParseFiles("templates/base.html", pageTemplate)
    if err != nil {
        http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
        return
    }

    if err := tmpl.ExecuteTemplate(w, "base.html", td); err != nil {
        http.Error(w, "Render error: "+err.Error(), http.StatusInternalServerError)
    }
}

// renderPartial renders a template WITHOUT base.html.
// Used for HTMX endpoints that return HTML fragments, not full pages.
//
// Usage:
//
//	renderPartial(w, someData, "templates/partials/search_results.html")
func renderPartial(w http.ResponseWriter, data any, partialTemplate string) {
    tmpl, err := template.New("").Funcs(templateFuncs).ParseFiles(partialTemplate)
    if err != nil {
        http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
        return
    }

	// We ExecuteTemplate by the filename without path, e.g. "search_results.html"
	// because that's how Go names parsed templates — by their base filename
	if err := tmpl.Execute(w, data); err != nil {
        http.Error(w, "Render error: "+err.Error(), http.StatusInternalServerError)
    }
}
