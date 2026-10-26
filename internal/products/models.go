package products

type Product struct {
	ID         int    `json:"id"`
	CategoryID int    `json:"category_id"`
	Price      int    `json:"price"`
	Name       string `json:"name"`
}

// ProductDetailResponse, ürün detay sayfasında kullanılacak.
type ProductDetailResponse struct {
	ID          int      `json:"id"`          // Add To Cart için gereklidir
	Name        string   `json:"name"`        // Okunaklı ürün adı
	Description string   `json:"description"` // Okunaklı ürün açıklaması
	Price       int      `json:"price"`       // Kuruş bazında
	Stock       int      `json:"stock"`       // Stok adedi
	Category    string   `json:"category"`    // Okunaklı kategori adı
	Images      []string `json:"images"`      // Birden fazla resim URL'si olacağı için dizi
	Video       *string  `json:"video"`       // Her üründe video olmayabilir, null dönebilmesi için pointer
}

// Pagination struct'ı listeleme isteklerindeki sayfa ve limit verilerini temsil eder.
type Pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
	Pages int `json:"pages"`
}

// PaginatedResponse, API'den dönülecek olan standart veri listeleme formatıdır.
// T any diyerek "Bu yapıya her türlü listeyi (Product, Category, Order) verebilirim" diyoruz.
type PaginatedResponse[T any] struct {
	Data       []T        `json:"data"` 
	Pagination Pagination `json:"pagination"`
}

// ProductFilter, ürün aramalarında kullanılacak filtreleme kriterlerini tutar.
type ProductFilter struct {
	Locale   string
	Search   string
	Category string
	MinPrice string
	MaxPrice string
	Sort     string
	Page     int
	Limit    int
}

// Admin tarafındaki güncellemeler için istek modelleri
type UpdateProductRequest struct {
	CategoryID int    `json:"category_id"`
	Price      int    `json:"price"`
	Stock      int    `json:"stock"`
	Status     string `json:"status"`
}

type UpdatePriceRequest struct {
	Price int `json:"price"`
}

type UpdateStockRequest struct {
	Stock int `json:"stock"`
}