package products

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Handler, products paketinin HTTP işleyicilerini ve bağımlılıklarını taşır.
type Handler struct {
	DB  *pgxpool.Pool
	RDB *redis.Client
}

// NewHandler, products.Handler oluşturup geri döndürür.
func NewHandler(db *pgxpool.Pool, rdb *redis.Client) *Handler {
	return &Handler{DB: db, RDB: rdb}
}

// GetProducts, HTTP üzerinden gelen ürün listeleme isteklerini karşılar.
// Query parametrelerini okur, veritabanına iletir ve sonucu JSON formatında sayfalı olarak döner.
func (h *Handler) GetProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// 1. Dil okuma (ÖNEMLİ: Varsayılan dil en yapılmalı)
	locale := q.Get("locale")
	if locale == "" {
		locale = "en"
	}

	// 2. Sayfalama parametreleri parse etme
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 20
	}

	// 3. Filtre nesnesini oluşturma (Veritabanına temiz bir şekilde iletmek için)
	filter := ProductFilter{
		Locale:   locale,
		Search:   q.Get("search"),
		Category: q.Get("category"),
		MinPrice: q.Get("min_price"),
		MaxPrice: q.Get("max_price"),
		Sort:     q.Get("sort"),
		Page:     page,
		Limit:    limit,
	}

	// 4. Veritabanından veriyi çekme (SRP - Single Responsibility Principle gereği veritabanı işi repository.go'da)
	products, total, err := GetProductsList(r.Context(), h.DB, filter)
	if err != nil {
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	// 5. Sayfa sayısı hesaplama
	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	// 6. JSON yanıtı hazırlama
	response := PaginatedResponse[Product]{
		Data: products,
		Pagination: Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
			Pages: pages,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProductDetail, URL'den alınan slug'a göre tek bir ürünün detaylarını getirir.
// Veritabanına gitmeden önce yüksek performans için Redis Cache üzerinden kontrol yapar.
func (h *Handler) GetProductDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.Error(w, "Invalid product slug/Geçersiz ürün slug'ı", http.StatusBadRequest)
		return
	}

	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = "en"
	}

	// Redis Cache Logic: Her ürün ve her dil için ayrı bir cache anahtarı (Örn: product:iphone-15:tr)
	cacheKey := "product:" + slug + ":" + locale

	// 1. Önce Redis cache'den veriyi okumayı dene (Performans artışı için)
	cachedData, err := h.RDB.Get(r.Context(), cacheKey).Result()
	if err == nil && cachedData != "" {
		// Cache'de veri bulundu, doğrudan JSON olarak dön ve işlemi bitir
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedData))
		return
	}

	// 2. Cache'de yoksa, veritabanından güncel veriyi çek
	product, err := GetProductDetailBySlug(r.Context(), h.DB, slug, locale)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Ürün bulunamazsa özel olarak 404 Not Found dön
			http.Error(w, "Product not found/Ürün bulunamadı", http.StatusNotFound)
			return
		}
		http.Error(w, "Database query error/Veritabanı sorgu hatası", http.StatusInternalServerError)
		return
	}

	productJson, marshalErr := json.Marshal(product)
	if marshalErr != nil {
		http.Error(w, "JSON marshal error/JSON dönüştürme hatası", http.StatusInternalServerError)
		return
	}

	// 3. Çekilen güncel veriyi bir sonraki istekler için Redis cache'e kaydet (10 dakika TTL - Yaşam süresi)
	err = h.RDB.Set(r.Context(), cacheKey, productJson, 10*time.Minute).Err()
	if err != nil {
		log.Printf("Failed to set cache for product %s: %v", slug, err)
	}

	// 4. Sonucu dön
	w.Header().Set("Content-Type", "application/json")
	w.Write(productJson)
}

// InvalidateProductCache, bir ürünün Redis cache'indeki tüm varyasyonlarını (farklı diller için olanlar dahil) temizler.
func (h *Handler) InvalidateProductCache(ctx context.Context, slug string) error {
	var cursor uint64
	pattern := "product:" + slug + ":*"

	for {
		// Redis'i bloke etmemek için KEYS yerine Scan komutunu kullanıyorum.
		// Bu pattern'e uyan tüm anahtarları (örn: product:iphone-15:en, product:iphone-15:tr) parça parça getirir.

		keys, nextCursor, err := h.RDB.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		// Eğer pattern'e uyan cache (anahtarlar) bulunursa, bunları temizliyorum.

		if len(keys) > 0 {
			if err := h.RDB.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor
		// Cursor 0 olduysa tüm cache (anahtarlar) taranmış demektir. Dögüden çıkıyorum

		if nextCursor == 0 {
			break
		}
	}
	log.Printf("Ürün için önbellek geçersiz kılındı/Cache invalidated for product %s", slug)
	return nil
}

// UpdateProduct, Admin'in ürünü güncellemesini sağlar ve cache'i temizler.
func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id == 0 {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var req UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	slug, err := UpdateProduct(r.Context(), h.DB, id, req.CategoryID, req.Price, req.Stock, req.Status)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// İlgili cache'i temizle
	h.InvalidateProductCache(r.Context(), slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Product updated successfully"})
}

// UpdatePrice, Admin'in ürün fiyatını güncellemesini sağlar ve cache'i temizler.
func (h *Handler) UpdatePrice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id == 0 {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var req UpdatePriceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	slug, err := UpdatePrice(r.Context(), h.DB, id, req.Price)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// İlgili cache'i temizle
	h.InvalidateProductCache(r.Context(), slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Price updated successfully"})
}

// UpdateStock, Admin'in ürün stokunu güncellemesini sağlar ve cache'i temizler.
func (h *Handler) UpdateStock(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id == 0 {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var req UpdateStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	slug, err := UpdateStock(r.Context(), h.DB, id, req.Stock)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// İlgili cache'i temizle
	h.InvalidateProductCache(r.Context(), slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Stock updated successfully"})
}

// ArchiveProduct, Admin'in ürünü arşivlemesini sağlar ve cache'i temizler.
func (h *Handler) ArchiveProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id == 0 {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	slug, err := ArchiveProduct(r.Context(), h.DB, id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// İlgili cache'i temizle
	h.InvalidateProductCache(r.Context(), slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Product archived successfully"})
}

// DeleteProduct, Admin'in ürünü silmesini sağlar ve cache'i temizler.
func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id == 0 {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	slug, err := DeleteProduct(r.Context(), h.DB, id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// İlgili cache'i temizle
	h.InvalidateProductCache(r.Context(), slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Product deleted successfully"})
}