package routes

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/jasonmichels/Market-Sentry/internal/auth"
	"github.com/jasonmichels/Market-Sentry/internal/storage"
)

// RegisterAdminRoutes registers admin routes.
func RegisterAdminRoutes(mux *http.ServeMux, store *storage.MemoryStore, adminPhones map[string]bool) {
	mux.Handle("/admin", auth.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAdmin(store, w, r, adminPhones)
	})))
}

func handleAdmin(store *storage.MemoryStore, w http.ResponseWriter, r *http.Request, adminPhones map[string]bool) {
	phone := auth.GetUserPhone(r.Context())
	if !adminPhones[phone] {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	pageStr := r.URL.Query().Get("page")
	pageNum, err := strconv.Atoi(pageStr)
	if err != nil || pageNum < 1 {
		pageNum = 1
	}
	const pageSize = 100

	store.Mu.RLock()
	userList := make([]*storage.User, 0, len(store.Users))
	for _, u := range store.Users {
		userList = append(userList, u)
	}
	totalUsers := len(userList)
	store.Mu.RUnlock()

	start := (pageNum - 1) * pageSize
	if start > totalUsers {
		start = totalUsers
	}
	end := start + pageSize
	if end > totalUsers {
		end = totalUsers
	}
	pageUsers := userList[start:end]

	data := struct {
		CurrentPage int
		PageSize    int
		TotalUsers  int
		Users       []*storage.User
	}{
		CurrentPage: pageNum,
		PageSize:    pageSize,
		TotalUsers:  totalUsers,
		Users:       pageUsers,
	}

	funcs := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
		"mul": func(a, b int) int { return a * b },
	}
	tmpl := template.Must(template.New("admin.html").Funcs(funcs).ParseFiles("web/templates/admin.html"))
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error executing admin template: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}
