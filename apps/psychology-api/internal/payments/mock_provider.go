package payments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// MockPaymentProvider is an in-memory test adapter for payment gateway operations.
type MockPaymentProvider struct {
	mu                       sync.Mutex
	RequestCalls             []PaymentRequest
	VerifyCalls              []VerifyRequest
	SimulateTimeout          bool
	SimulateNetworkError     bool
	SimulateGatewayRejection bool
	GatewayErrorMessage      string
	FixedAuthority           string
	CustomRequestHandler     func(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
	CustomVerifyHandler      func(ctx context.Context, req VerifyRequest) (*VerifyResponse, error)
}

func NewMockPaymentProvider() *MockPaymentProvider {
	return &MockPaymentProvider{
		RequestCalls: make([]PaymentRequest, 0),
		VerifyCalls:  make([]VerifyRequest, 0),
	}
}

func (m *MockPaymentProvider) Name() string {
	return "mock"
}

func (m *MockPaymentProvider) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RequestCalls = append(m.RequestCalls, req)

	if m.CustomRequestHandler != nil {
		return m.CustomRequestHandler(ctx, req)
	}

	if m.SimulateTimeout {
		return nil, ErrGatewayTimeout
	}

	if m.SimulateNetworkError {
		return nil, ErrGatewayUnavailable
	}

	if m.SimulateGatewayRejection {
		return nil, ErrGatewayRejected
	}

	authority := m.FixedAuthority
	if authority == "" {
		bytes := make([]byte, 16)
		_, _ = rand.Read(bytes)
		authority = "mock_auth_" + hex.EncodeToString(bytes)
	}

	paymentURL := "https://test-pay.example.com/mock-gateway?authority=" + authority

	return &PaymentResponse{
		Authority:  authority,
		PaymentURL: paymentURL,
		FeeRials:   0,
	}, nil
}

func (m *MockPaymentProvider) VerifyPayment(ctx context.Context, req VerifyRequest) (*VerifyResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.VerifyCalls = append(m.VerifyCalls, req)

	if m.CustomVerifyHandler != nil {
		return m.CustomVerifyHandler(ctx, req)
	}

	if m.SimulateTimeout {
		return nil, ErrGatewayTimeout
	}

	if m.SimulateNetworkError {
		return nil, ErrGatewayUnavailable
	}

	if m.SimulateGatewayRejection {
		return nil, ErrGatewayRejected
	}

	return &VerifyResponse{
		RefID:    "mock_ref_12345678",
		Code:     100,
		FeeRials: 0,
		CardPan:  "502229******1234",
		CardHash: "mock_card_hash_xyz",
	}, nil
}
