package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Racemir/e-commerce/internal/auth"
	"github.com/Racemir/e-commerce/internal/cart"
	"github.com/Racemir/e-commerce/internal/categories"
	"github.com/Racemir/e-commerce/internal/orders"
	"github.com/Racemir/e-commerce/internal/products"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Sağlık kontrolü için json kalıbı
type Healthresponse struct {
	Status string `json:"status"`
}

// Mux oluşturup geri dönene ana fonksiyon
func SetupRoutes(db *pgxpool.Pool, rdb *redis.Client) *http.ServeMux {
	// Gelen HTTP isteklerinin URL yolarına bakarak fonksiyonlara yönlendirir.
	mux := http.NewServeMux()
	authHandler := auth.NewHandler(db, rdb)
	ordersHandler := orders.NewHandler(db)
	productHandler := products.NewHandler(db, rdb)
	categoryHandler := categories.NewHandler(db)
	cartHandler := cart.NewHandler(db)
	adminMux := http.NewServeMux()

	// Normal Rotalar
	mux.HandleFunc("GET /", homePageHandler)
	mux.HandleFunc("GET /api/health", HealthHandler)
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("GET /api/auth/verify-email", authHandler.VerifyEmail)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/reset-password", authHandler.ResetPassword)
	mux.HandleFunc("POST /api/auth/forgot-password", authHandler.ForgotPassword)
	mux.HandleFunc("POST /api/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("GET /api/categories", categoryHandler.GetListCategories)
	mux.HandleFunc("GET /api/products", productHandler.GetProducts)
	mux.HandleFunc("GET /api/products/{slug}", productHandler.GetProductDetail)

	// Korumalı Rotalar (Kullanıcı Oturumu Gerektirenler)
	mux.Handle("GET /api/auth/me", authHandler.RequireAuth(http.HandlerFunc(authHandler.Me)))
	mux.Handle("GET /api/auth/logout", authHandler.RequireAuth(http.HandlerFunc(authHandler.Logout)))

	// Sepet Rotaları (Oturum korumalı)
	mux.Handle("GET /api/cart", authHandler.RequireAuth(http.HandlerFunc(cartHandler.GetCart)))
	mux.Handle("POST /api/cart/items", authHandler.RequireAuth(http.HandlerFunc(cartHandler.AddItem)))
	mux.Handle("PUT /api/cart/items/{id}", authHandler.RequireAuth(http.HandlerFunc(cartHandler.UpdateItem)))
	mux.Handle("DELETE /api/cart/items/{id}", authHandler.RequireAuth(http.HandlerFunc(cartHandler.DeleteItem)))

	// Sipariş Rotaları (Resource Ownership korumalı)
	mux.Handle("GET /api/orders/{id}", authHandler.RequireAuth(http.HandlerFunc(ordersHandler.GetOrder)))

	// Admin Rotaları
	mux.Handle("/api/admin/", authHandler.RequireAuth(auth.AdminOnly(adminMux)))

	adminMux.HandleFunc("POST /api/admin/categories", categoryHandler.CreateCategory)
	adminMux.HandleFunc("PUT /api/admin/categories/{id}", categoryHandler.UpdateCategory)
	adminMux.HandleFunc("DELETE /api/admin/categories/{id}", categoryHandler.DeleteCategory)

	adminMux.HandleFunc("PUT /api/admin/products/{id}", productHandler.UpdateProduct)
	adminMux.HandleFunc("PATCH /api/admin/products/{id}/price", productHandler.UpdatePrice)
	adminMux.HandleFunc("PATCH /api/admin/products/{id}/stock", productHandler.UpdateStock)
	adminMux.HandleFunc("PATCH /api/admin/products/{id}/archive", productHandler.ArchiveProduct)
	adminMux.HandleFunc("DELETE /api/admin/products/{id}", productHandler.DeleteProduct)

	return mux
}

// Anasayfa fonksiyonu
func homePageHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "E-commerce Application")
}

// Health endpoint fonksiyonu
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	// Header content-type verinin fomratı nedir
	// application: binary veya metin verimiz json formatında
	w.Header().Set("Content-Type", "application/json")
	healthData := Healthresponse{Status: "ok"}

	// NewEncoder veriyi doğrudan HTTP yanıtına (w) yazacak bir dönüştürücü oluşturur.
	// Encode healthData isimli Go yapısını alır, JSON'a çevirir ve (w) üzerinden gönderir.
	// Veriyi bellekte bir değişkene atamadık RAM'i yormadık doğrudan stream (akış) olarak gönderdik.
	json.NewEncoder(w).Encode(healthData)
}
