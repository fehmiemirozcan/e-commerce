package categories

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetCategory, kullanıcının diline (locale) göre tüm kategorileri veritabanından çeker ve listeler.
func GetCategory(ctx context.Context, db *pgxpool.Pool, locale string) ([]Category, error) {

	query := `SELECT categories.id, category_translations.name 
	FROM categories 
	INNER JOIN category_translations ON categories.id = category_translations.category_id
	WHERE category_translations.locale = $1`

	rows, execError := db.Query(ctx, query, locale)
	if execError != nil {
		return []Category{}, execError
	}
	defer rows.Close()

	var categoryList []Category

	for rows.Next() {
		var category Category
		if scanError := rows.Scan(&category.ID, &category.Name); scanError != nil {
			return nil, scanError
		}
		categoryList = append(categoryList, category)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categoryList, nil
}

// CreateCategory, sisteme yepyeni bir kategori ve onun ilk dil çevirisini ekler.
func CreateCategory(ctx context.Context, db *pgxpool.Pool, name, locale string) (int, error) {
	query := `INSERT INTO categories (id) VALUES (DEFAULT) RETURNING id`

	var newId int
	err := db.QueryRow(ctx, query).Scan(&newId)
	if err != nil {
		return 0, err
	}

	insertTranslationQuery := `INSERT INTO category_translations (category_id, locale, name) VALUES ($1, $2, $3)`
	
	_, insertErr := db.Exec(ctx, insertTranslationQuery, newId, locale, name)
	if insertErr != nil {
		return 0, insertErr
	}
	
	return newId, nil
}

// UpdateCategory, mevcut bir kategorinin belirli bir dildeki ismini değiştirir.
func UpdateCategory(ctx context.Context, db *pgxpool.Pool, categoryId int, newCategoryName string, locale string) error {
	query := `UPDATE category_translations SET name = $1
	WHERE category_id = $2 AND locale = $3`

	_, err := db.Exec(ctx, query, newCategoryName, categoryId, locale)
	return err
}

// DeleteCategory, bir kategoriyi sistemden tamamen siler.
func DeleteCategory(ctx context.Context,db *pgxpool.Pool, categoryId int) error {
	query := `DELETE FROM categories WHERE id = $1`

	_,err := db.Exec(ctx, query, categoryId)
	return err
}
