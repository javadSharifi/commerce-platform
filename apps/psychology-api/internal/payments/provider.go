package payments

import (
	"context"
	"errors"
)

var (
	ErrGatewayTimeout            = errors.New("payments: gateway timeout while requesting payment")
	ErrGatewayUnavailable        = errors.New("payments: payment gateway is temporarily unavailable")
	ErrGatewayRejected           = errors.New("payments: payment gateway rejected request")
	ErrInvalidAmount             = errors.New("payments: amount must be positive integer in Rials")
	ErrZeroOrNegativeAmount      = ErrInvalidAmount
	ErrPurchaseNotFound          = errors.New("payments: purchase not found")
	ErrRegistrationNotFound      = errors.New("payments: registration not found")
	ErrRegistrationExpired       = errors.New("payments: registration has expired")
	ErrInvalidRegistrationStatus = errors.New("payments: registration is not pending payment")
	ErrPaymentAlreadyVerified    = errors.New("payments: payment has already been verified")
	ErrActiveAttemptInProgress   = errors.New("payments: an active payment attempt is already in progress for this purchase")
	ErrInvalidIdempotencyKey     = errors.New("payments: idempotency key cannot be empty")
	ErrWorkshopNotPublished      = errors.New("payments: workshop is not open for registration")
	ErrWorkshopNotFound          = errors.New("payments: workshop not found")
	ErrPaymentAttemptNotFound         = errors.New("payments: payment attempt not found")
	ErrAmountMismatch                 = errors.New("payments: payment amount mismatch")
	ErrInvalidAuthority               = errors.New("payments: authority cannot be empty")
	ErrDuplicateRefundReference       = errors.New("payments: refund reference number already exists")
	ErrPurchaseAlreadyRefunded        = errors.New("payments: purchase is already refunded")
	ErrInvalidPurchaseStatusForRefund = errors.New("payments: purchase is not in refund_required status")
	ErrCourseAlreadyEntitled          = errors.New("payments: شما قبلاً در این دوره ثبت‌نام کرده‌اید و دسترسی شما فعال است")
)

// PaymentRequest contains parameters for initiating a payment transaction.
type PaymentRequest struct {
	AmountRials int64
	Description string
	CallbackURL string
	Mobile      string
	Email       string
}

// PaymentResponse contains the gateway's response for payment initiation.
type PaymentResponse struct {
	Authority  string
	PaymentURL string
	FeeRials   int64
}

// VerifyRequest contains parameters for verifying a completed payment.
type VerifyRequest struct {
	Authority   string
	AmountRials int64
}

// VerifyResponse contains the gateway's verification result.
type VerifyResponse struct {
	RefID    string
	Code     int
	FeeRials int64
	CardPan  string
	CardHash string
}

// PaymentProvider defines the interface for interacting with payment gateways.
type PaymentProvider interface {
	Name() string
	RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
	VerifyPayment(ctx context.Context, req VerifyRequest) (*VerifyResponse, error)
}
