package orders

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)



// GetOrderByIDAndUser, SQL sorgusunu çalıştırarak siparişi getirir.
func GetOrderByIDAndUser(ctx context.Context, db *pgxpool.Pool, orderID int, userID int) (Order, error) {
	query := `
		SELECT id, user_id, total_amount, currency, status 
		FROM orders 
		WHERE id = $1 AND user_id = $2
	`

	var order Order
	err := db.QueryRow(ctx, query, orderID, userID).Scan(
		&order.ID,
		&order.UserID,
		&order.TotalAmount,
		&order.Currency,
		&order.Status,
	)

	return order, err
}
