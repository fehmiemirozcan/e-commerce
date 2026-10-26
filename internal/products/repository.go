package products

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetProductsList, ürünlerin listelenmesi ve filtrelenmesi işlemini veritabanında gerçekleştirir.
// Gelen filtrelere göre SQL sorgusunu dinamik olarak oluşturur ve ürün listesi ile toplam kayıt sayısını döner.
func GetProductsList(ctx context.Context, db *pgxpool.Pool, filter ProductFilter) ([]Product, int, error) {
	// Offset hesaplaması (Hangi sayfadaysak o kadar satır atla)
	offset := (filter.Page - 1) * filter.Limit

	// Temel SQL Sorgumuz
	baseQuery := `
		FROM products p
		INNER JOIN product_translations pt ON p.id = pt.product_id AND pt.locale = $1
		WHERE 1=1
	`
	var args []any
	args = append(args, filter.Locale)
	argCount := 2

	// Dinamik filtre ekleme
	if filter.Search != "" {
		baseQuery += fmt.Sprintf(" AND pt.name ILIKE $%d ", argCount)
		args = append(args, "%"+filter.Search+"%")
		argCount++
	}
	if filter.Category != "" {
		baseQuery += fmt.Sprintf(" AND p.category_id = $%d ", argCount)
		args = append(args, filter.Category)
		argCount++
	}
	if filter.MinPrice != "" {
		baseQuery += fmt.Sprintf(" AND p.price >= $%d ", argCount)
		args = append(args, filter.MinPrice)
		argCount++
	}
	if filter.MaxPrice != "" {
		baseQuery += fmt.Sprintf(" AND p.price <= $%d ", argCount)
		args = append(args, filter.MaxPrice)
		argCount++
	}

	// 1. İşlem: Toplam ürün sayısını bul (Pagination için gerekli)
	countQuery := "SELECT COUNT(p.id) " + baseQuery
	var total int
	err := db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 2. İşlem: İstenilen sayfadaki ürünleri getir
	selectQuery := "SELECT p.id, p.category_id, p.price, pt.name " + baseQuery

	if filter.Sort != "" {
		switch filter.Sort {
		case "price_asc":
			selectQuery += " ORDER BY p.price ASC "
		case "price_desc":
			selectQuery += " ORDER BY p.price DESC "
		case "newest":
			selectQuery += " ORDER BY p.created_at DESC "
		case "oldest":
			selectQuery += " ORDER BY p.created_at ASC "
		default:
			selectQuery += " ORDER BY p.created_at DESC "
		}
	} else {
		selectQuery += " ORDER BY p.created_at DESC "
	}

	selectQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d ", argCount, argCount+1)
	args = append(args, filter.Limit, offset)

	rows, err := db.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		if err := rows.Scan(&product.ID, &product.CategoryID, &product.Price, &product.Name); err != nil {
			return nil, 0, err
		}
		products = append(products, product)
	}

	// products nil ise boş dizi yap (JSON'da null dönmemesi için)
	if products == nil {
		products = []Product{}
	}

	return products, total, nil
}

// GetProductDetailBySlug, ürün detay sayfasında kullanılacak ürünü veritabanından getirir.
func GetProductDetailBySlug(ctx context.Context, db *pgxpool.Pool, slug string, locale string) (ProductDetailResponse, error) {
	query := `
		SELECT 
			p.id, 
			p.price, 
			pr.name, 
			pr.description, 
			p.stock, 
			ct.name as category
		FROM products p
		INNER JOIN product_translations pr ON p.id = pr.product_id
		INNER JOIN categories c ON p.category_id = c.id
		LEFT JOIN category_translations ct ON c.id = ct.category_id AND ct.locale = $2
		WHERE p.slug = $1 AND pr.locale = $2
	`

	var product ProductDetailResponse
	err := db.QueryRow(ctx, query, slug, locale).Scan(
		&product.ID,
		&product.Price,
		&product.Name,
		&product.Description,
		&product.Stock,
		&product.Category,
	)

	if err != nil {
		return ProductDetailResponse{}, err
	}

	return product, nil
}

// UpdateProduct ürünü günceller ve güncellenen ürünün slug'ını döner.
func UpdateProduct(ctx context.Context, db *pgxpool.Pool, id int, categoryID, price, stock int, status string) (string, error) {
	var slug string
	query := `
		UPDATE products 
		SET category_id = $1, price = $2, stock = $3, status = $4, updated_at = CURRENT_TIMESTAMP
		WHERE id = $5 
		RETURNING slug
	`
	err := db.QueryRow(ctx, query, categoryID, price, stock, status, id).Scan(&slug)
	return slug, err
}

// UpdatePrice ürün fiyatını günceller ve güncellenen ürünün slug'ını döner.
func UpdatePrice(ctx context.Context, db *pgxpool.Pool, id int, price int) (string, error) {
	var slug string
	query := `UPDATE products SET price = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 RETURNING slug`
	err := db.QueryRow(ctx, query, price, id).Scan(&slug)
	return slug, err
}

// UpdateStock ürün stokunu günceller ve güncellenen ürünün slug'ını döner.
func UpdateStock(ctx context.Context, db *pgxpool.Pool, id int, stock int) (string, error) {
	var slug string
	query := `UPDATE products SET stock = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 RETURNING slug`
	err := db.QueryRow(ctx, query, stock, id).Scan(&slug)
	return slug, err
}

// ArchiveProduct ürünü arşivler ve güncellenen ürünün slug'ını döner.
func ArchiveProduct(ctx context.Context, db *pgxpool.Pool, id int) (string, error) {
	var slug string
	query := `UPDATE products SET status = 'archived', updated_at = CURRENT_TIMESTAMP WHERE id = $1 RETURNING slug`
	err := db.QueryRow(ctx, query, id).Scan(&slug)
	return slug, err
}

// DeleteProduct ürünü siler ve silinen ürünün slug'ını döner.
func DeleteProduct(ctx context.Context, db *pgxpool.Pool, id int) (string, error) {
	var slug string
	query := `DELETE FROM products WHERE id = $1 RETURNING slug`
	err := db.QueryRow(ctx, query, id).Scan(&slug)
	return slug, err
}