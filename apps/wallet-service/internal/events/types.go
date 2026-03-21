package events

// Event types
const (
	EventReserveFunds      = "ReserveFunds"
	EventFundsReserved     = "FundsReserved"
	EventInsufficientFunds = "InsufficientFunds"
	EventReleaseFunds      = "ReleaseFunds"
	EventFundsReleased     = "FundsReleased"
)

// ReserveFundsEvent is sent by Payment Service
type ReserveFundsEvent struct {
	PaymentID string  `json:"payment_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Timestamp string  `json:"timestamp"`
}

// FundsReservedEvent is sent back to Payment Service
type FundsReservedEvent struct {
	PaymentID string `json:"payment_id"`
	Timestamp string `json:"timestamp"`
}

// InsufficientFundsEvent is sent back to Payment Service
type InsufficientFundsEvent struct {
	PaymentID string `json:"payment_id"`
	UserID    string `json:"user_id"`
	Timestamp string `json:"timestamp"`
}

// ReleaseFundsEvent is sent by Gateway Service (compensation)
type ReleaseFundsEvent struct {
	PaymentID string `json:"payment_id"`
	Timestamp string `json:"timestamp"`
}

// FundsReleasedEvent confirms the release
type FundsReleasedEvent struct {
	PaymentID string `json:"payment_id"`
	Timestamp string `json:"timestamp"`
}
