package orders

// Order yapısı, veritabanındaki sipariş satırını temsil eder.
type Order struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
	TotalAmount int    `json:"total_amount"` // Kuruş bazında
	Currency    string `json:"currency"`
	Status      string `json:"status"`
}
