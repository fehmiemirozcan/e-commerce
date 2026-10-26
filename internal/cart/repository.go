package cart

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetOrCreateCart, kullanıcının sepeti varsa id'sini döner, yoksa yeni bir sepet oluşturur.
func GetOrCreateCart(ctx context.Context, db *pgxpool.Pool, userID int) (int, error) {
	var cartId int
	query := `SELECT id FROM carts WHERE user_id = $1`
	err := db.QueryRow(ctx, query, userID).Scan(&cartId)

	if err == pgx.ErrNoRows {
		insertQuery := `INSERT INTO carts (user_id) VALUES ($1) RETURNING id`
		err = db.QueryRow(ctx, insertQuery, userID).Scan(&cartId)
		if err != nil {
			return 0, err
		}
		return cartId, nil
	} else if err != nil {
		return 0, err
	}

	return cartId, nil
}

// GetCartWithItems, kullanıcının sepetindeki ürünleri, güncel veritabanı fiyatını ve lokalize edilmiş adını döner.
func GetCartWithItems(ctx context.Context, db *pgxpool.Pool, cartID int, locale string) (CartResponse, error) {
	query := `
		SELECT 
			ci.id AS item_id,
			ci.product_id,
			ci.quantity,
			p.price,
			COALESCE(pt.name, p.slug) AS name
		FROM cart_items ci
		JOIN products p ON ci.product_id = p.id
		LEFT JOIN product_translations pt ON p.id = pt.product_id AND pt.locale = $2
		WHERE ci.cart_id = $1
		ORDER BY ci.id ASC
	`

	rows, err := db.Query(ctx, query, cartID, locale)
	if err != nil {
		return CartResponse{}, err
	}
	defer rows.Close()

	items := make([]CartItemResponse, 0)
	totalAmount := 0

	for rows.Next() {
		var item CartItemResponse
		if err := rows.Scan(&item.ItemID, &item.ProductID, &item.Quantity, &item.Price, &item.Name); err != nil {
			return CartResponse{}, err
		}
		item.Subtotal = item.Price * item.Quantity
		totalAmount += item.Subtotal
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return CartResponse{}, err
	}

	return CartResponse{
		CartID:      cartID,
		Items:       items,
		TotalAmount: totalAmount,
	}, nil
}

// AddItemToCart, sepete ürün ekler veya mevcut ürünün miktarını artırır.
func AddItemToCart(ctx context.Context, db *pgxpool.Pool, cartID int, productID int, quantity int) error {
	var stock int
	err := db.QueryRow(ctx, "SELECT stock FROM products WHERE id = $1 AND status = 'active'", productID).Scan(&stock)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("product not found or inactive")
		}
		return err
	}

	var existingID int
	var existingQty int
	err = db.QueryRow(ctx, "SELECT id, quantity FROM cart_items WHERE cart_id = $1 AND product_id = $2", cartID, productID).Scan(&existingID, &existingQty)
	if errors.Is(err, pgx.ErrNoRows) {
		if stock < quantity {
			return errors.New("insufficient stock")
		}
		_, err = db.Exec(ctx, "INSERT INTO cart_items (cart_id, product_id, quantity) VALUES ($1, $2, $3)", cartID, productID, quantity)
		return err
	} else if err != nil {
		return err
	}

	if stock < existingQty+quantity {
		return errors.New("insufficient stock")
	}

	_, err = db.Exec(ctx, "UPDATE cart_items SET quantity = quantity + $1 WHERE id = $2", quantity, existingID)
	return err
}

// UpdateCartItemQuantity, sepetteki bir ürünün miktarını günceller.
func UpdateCartItemQuantity(ctx context.Context, db *pgxpool.Pool, cartID int, itemID int, quantity int) error {
	var stock int
	err := db.QueryRow(ctx, `
		SELECT p.stock 
		FROM cart_items ci
		JOIN products p ON ci.product_id = p.id
		WHERE ci.id = $1 AND ci.cart_id = $2
	`, itemID, cartID).Scan(&stock)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("cart item not found")
		}
		return err
	}

	if stock < quantity {
		return errors.New("insufficient stock")
	}

	_, err = db.Exec(ctx, "UPDATE cart_items SET quantity = $1 WHERE id = $2 AND cart_id = $3", quantity, itemID, cartID)
	return err
}

// RemoveCartItem, sepetteki ürünü siler.
func RemoveCartItem(ctx context.Context, db *pgxpool.Pool, cartID int, itemID int) error {
	res, err := db.Exec(ctx, "DELETE FROM cart_items WHERE id = $1 AND cart_id = $2", itemID, cartID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return errors.New("cart item not found")
	}
	return nil
}