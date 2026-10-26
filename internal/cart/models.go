package cart

type CartItemResponse struct {
	ItemID    int    `json:"item_id"`
	ProductID int    `json:"product_id"`
	Name      string `json:"name"`
	Price     int    `json:"price"` // Kuruş cinsinden (products.price)
	Quantity  int    `json:"quantity"`
	Subtotal  int    `json:"subtotal"`
}

type CartResponse struct {
	CartID      int                `json:"cart_id"`
	Items       []CartItemResponse `json:"items"`
	TotalAmount int                `json:"total_amount"`
}

type AddItemRequest struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

type UpdateItemRequest struct {
	Quantity int `json:"quantity"`
}
