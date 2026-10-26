package orders

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Racemir/e-commerce/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler, orders paketinin HTTP işleyicilerini ve bağımlılıklarını taşır.
type Handler struct {
	DB *pgxpool.Pool
}

// NewHandler, orders.Handler oluşturup geri döndürür.
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
}

// GetOrder, oturum açmış kullanıcının belirli bir siparişini döndürür.
// Resource Ownership Kontrolü:
//
//	JWT middleware tarafından context'e yazılan userID ile
//	URL'den okunan orderID birlikte sorgulanır.
//	Kullanıcı yalnızca kendi siparişine erişebilir (403 Forbidden).
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {

	// URL'den order ID'sini oku
	// Beklenen path: /api/orders/{id}
	// Go 1.22+ net/http ile r.PathValue("id") kullanılır.
	orderIDStr := r.PathValue("id")
	orderID, convErr := strconv.Atoi(orderIDStr)
	if convErr != nil || orderID <= 0 {
		http.Error(w, "Invalid order ID/Geçersiz sipariş ID'si", http.StatusBadRequest)
		return
	}

	// JWT context'inden kullanıcıyı oku (RequireAuth middleware tarafından set edildi)
	userRaw := r.Context().Value(auth.UserFromContext)
	if userRaw == nil {
		http.Error(w, "Unauthorized/Giriş yapılmamış", http.StatusUnauthorized)
		return
	}
	// Güvenli tip dönüşümü kontrolü (Type Assertion)
	currentUser, ok := userRaw.(*auth.User)
	if !ok {
		http.Error(w, "Internal Server Error/Kullanıcı bilgisi okunamadı", http.StatusInternalServerError)
		return
	}

	// Resource Ownership Kontrolü
	// Sorgu: orders tablosunda hem id=$1 hem user_id=$2 eşleşmeli.
	// Eşleşme yoksa pgx.ErrNoRows döner → bu siparişe erişim yasak (403).
	order, dbErr := GetOrderByIDAndUser(r.Context(), h.DB, orderID, currentUser.ID)
	if dbErr != nil {
		if errors.Is(dbErr, pgx.ErrNoRows) {
			// Kayıt yok ya da başkasının siparişi → 403
			http.Error(w, "Forbidden/Bu siparişe erişim yetkiniz yok", http.StatusForbidden)
			return
		}
		// Beklenmedik veritabanı hatası → 500
		http.Error(w, "Internal Server Error/Sunucu hatası", http.StatusInternalServerError)
		return
	}

	// --- 4. Başarılı: Siparişi JSON olarak döndür ---
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}