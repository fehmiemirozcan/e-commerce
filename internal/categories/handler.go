package categories

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	DB *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
}

// getLocale, HTTP isteğinden "locale" parametresini okur. Eğer boşsa varsayılan olarak "en" (İngilizce) döner.
func getLocale(r *http.Request) string {
	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = "en"
	}
	return locale
}

// GetListCategories, kullanıcıdan gelen dile göre tüm kategorileri listeler.
func (h *Handler) GetListCategories(w http.ResponseWriter, r *http.Request) {
	locale := getLocale(r)
	categories, err := GetCategory(r.Context(), h.DB, locale)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

// CreateCategory, gelen JSON objesine göre yepyeni bir kategori (ve o dile ait ilk çevirisini) oluşturur.
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var newCategory Category
	if err := json.NewDecoder(r.Body).Decode(&newCategory); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	locale := getLocale(r)
	newId, err := CreateCategory(r.Context(), h.DB, newCategory.Name, locale)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"id": newId})
}

// UpdateCategory, URL'den alınan ID'ye göre kategorinin belirtilen dildeki çeviri metnini günceller.
func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id == 0 {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	var updateCategory Category
	if err := json.NewDecoder(r.Body).Decode(&updateCategory); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	locale := getLocale(r)
	err = UpdateCategory(r.Context(), h.DB, id, updateCategory.Name, locale)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Category updated"})
}

// DeleteCategory, URL'den alınan ID'ye göre bir kategoriyi (ve ilgili tüm çevirilerini) sistemden siler.
func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id == 0 {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	err = DeleteCategory(r.Context(), h.DB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Category deleted"})
}

