package cart

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Racemir/e-commerce/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	DB *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
}

func getLocale(r *http.Request) string {
	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = "en"
	}
	return locale
}

func getUserID(r *http.Request) (int, bool) {
	userRaw := r.Context().Value(auth.UserFromContext)
	if userRaw == nil {
		return 0, false
	}
	currentUser, ok := userRaw.(*auth.User)
	if !ok || currentUser == nil {
		return 0, false
	}
	return currentUser.ID, true
}

// GetCart, giriş yapmış kullanıcının sepet içeriğini ve toplam tutarını döner.
func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	cartID, err := GetOrCreateCart(r.Context(), h.DB, userID)
	if err != nil {
		http.Error(w, "Failed to get cart", http.StatusInternalServerError)
		return
	}

	locale := getLocale(r)
	cartResponse, err := GetCartWithItems(r.Context(), h.DB, cartID, locale)
	if err != nil {
		http.Error(w, "Failed to load cart items", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cartResponse)
}

// AddItem, sepete ürün ekler.
func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProductID <= 0 || req.Quantity <= 0 {
		http.Error(w, "Product ID and Quantity must be positive integers", http.StatusBadRequest)
		return
	}

	cartID, err := GetOrCreateCart(r.Context(), h.DB, userID)
	if err != nil {
		http.Error(w, "Failed to get cart", http.StatusInternalServerError)
		return
	}

	if err := AddItemToCart(r.Context(), h.DB, cartID, req.ProductID, req.Quantity); err != nil {
		if err.Error() == "insufficient stock" || err.Error() == "product not found or inactive" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to add item to cart", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Item added to cart"})
}

// UpdateItem, sepetteki ürün adedini günceller.
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	itemID, err := strconv.Atoi(idStr)
	if err != nil || itemID <= 0 {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	var req UpdateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Quantity <= 0 {
		http.Error(w, "Quantity must be greater than zero", http.StatusBadRequest)
		return
	}

	cartID, err := GetOrCreateCart(r.Context(), h.DB, userID)
	if err != nil {
		http.Error(w, "Failed to get cart", http.StatusInternalServerError)
		return
	}

	if err := UpdateCartItemQuantity(r.Context(), h.DB, cartID, itemID, req.Quantity); err != nil {
		if err.Error() == "insufficient stock" || err.Error() == "cart item not found" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to update cart item", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Cart item updated"})
}

// DeleteItem, sepetten bir kalemi siler.
func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	itemID, err := strconv.Atoi(idStr)
	if err != nil || itemID <= 0 {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	cartID, err := GetOrCreateCart(r.Context(), h.DB, userID)
	if err != nil {
		http.Error(w, "Failed to get cart", http.StatusInternalServerError)
		return
	}

	if err := RemoveCartItem(r.Context(), h.DB, cartID, itemID); err != nil {
		if err.Error() == "cart item not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to remove cart item", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Cart item removed"})
}
