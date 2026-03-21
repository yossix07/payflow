package events

// Event types
const (
	EventPaymentStarted    = "PaymentStarted"
	EventReserveFunds      = "ReserveFunds"
	EventFundsReserved     = "FundsReserved"
	EventInsufficientFunds = "InsufficientFunds"
	EventProcessPayment    = "ProcessPayment"
	EventPaymentSucceeded  = "PaymentSucceeded"
	EventPaymentFailed     = "PaymentFailed"
	EventSendNotification  = "SendNotification"
	EventRecordTransaction = "RecordTransaction"
)

// Event structures
type PaymentStartedEvent struct {
	PaymentID string  `json:"payment_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Timestamp string  `json:"timestamp"`
}

type ReserveFundsEvent struct {
	PaymentID string  `json:"payment_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Timestamp string  `json:"timestamp"`
}

type FundsReservedEvent struct {
	PaymentID string `json:"payment_id"`
	Timestamp string `json:"timestamp"`
}

type InsufficientFundsEvent struct {
	PaymentID string `json:"payment_id"`
	UserID    string `json:"user_id"`
	Timestamp string `json:"timestamp"`
}

type ProcessPaymentEvent struct {
	PaymentID string  `json:"payment_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Timestamp string  `json:"timestamp"`
}

type PaymentSucceededEvent struct {
	PaymentID     string `json:"payment_id"`
	TransactionID string `json:"transaction_id"`
	Timestamp     string `json:"timestamp"`
}

type PaymentFailedEvent struct {
	PaymentID string `json:"payment_id"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
}

type SendNotificationEvent struct {
	PaymentID string  `json:"payment_id"`
	UserID    string  `json:"user_id"`
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
	Timestamp string  `json:"timestamp"`
}
