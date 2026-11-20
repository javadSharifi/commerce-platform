package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type ZarinPalProvider struct {
	merchantID      string
	isSandbox       bool
	httpClient      *http.Client
	requestEndpoint string
	startPayBaseURL string
	verifyEndpoint  string
}

type ZarinPalOption func(*ZarinPalProvider)

func WithBaseURL(base string) ZarinPalOption {
	return func(p *ZarinPalProvider) {
		base = strings.TrimRight(base, "/")
		p.requestEndpoint = base + "/pg/v4/payment/request.json"
		p.startPayBaseURL = base + "/pg/StartPay/"
		p.verifyEndpoint = base + "/pg/v4/payment/verify.json"
	}
}

func NewZarinPalProvider(merchantID string, isSandbox bool, client *http.Client, opts ...ZarinPalOption) *ZarinPalProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	p := &ZarinPalProvider{
		merchantID: merchantID,
		isSandbox:  isSandbox,
		httpClient: client,
	}

	if isSandbox {
		p.requestEndpoint = "https://sandbox.zarinpal.com/pg/v4/payment/request.json"
		p.startPayBaseURL = "https://sandbox.zarinpal.com/pg/StartPay/"
		p.verifyEndpoint = "https://sandbox.zarinpal.com/pg/v4/payment/verify.json"
	} else {
		p.requestEndpoint = "https://payment.zarinpal.com/pg/v4/payment/request.json"
		p.startPayBaseURL = "https://payment.zarinpal.com/pg/StartPay/"
		p.verifyEndpoint = "https://payment.zarinpal.com/pg/v4/payment/verify.json"
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *ZarinPalProvider) Name() string {
	return "zarinpal"
}

type zpMetadata struct {
	Mobile string `json:"mobile,omitempty"`
	Email  string `json:"email,omitempty"`
}

type zpPaymentReq struct {
	MerchantID  string     `json:"merchant_id"`
	Amount      int64      `json:"amount"`
	Description string     `json:"description"`
	CallbackURL string     `json:"callback_url"`
	Metadata    zpMetadata `json:"metadata"`
}

type zpData struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Authority string `json:"authority"`
	FeeType   string `json:"fee_type"`
	Fee       int64  `json:"fee"`
	RefID     any    `json:"ref_id"`
	CardPan   string `json:"card_pan"`
	CardHash  string `json:"card_hash"`
}

type zpResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors json.RawMessage `json:"errors"`
}

func parseZpData(raw json.RawMessage) *zpData {
	if len(raw) == 0 || raw[0] != '{' {
		return nil
	}
	var d zpData
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil
	}
	return &d
}

func parseZpError(errorsRaw json.RawMessage, data *zpData) (int, string) {
	if data != nil && data.Code != 0 {
		return data.Code, data.Message
	}
	if len(errorsRaw) > 0 {
		var errObj struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(errorsRaw, &errObj); err == nil && errObj.Code != 0 {
			return errObj.Code, errObj.Message
		}
		return 0, string(errorsRaw)
	}
	return 0, "unknown error"
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

func (p *ZarinPalProvider) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	if req.AmountRials <= 0 {
		return nil, ErrInvalidAmount
	}
	if req.CallbackURL == "" {
		return nil, errors.New("payments: callback_url cannot be empty")
	}

	payload := zpPaymentReq{
		MerchantID:  p.merchantID,
		Amount:      req.AmountRials,
		Description: req.Description,
		CallbackURL: req.CallbackURL,
		Metadata: zpMetadata{
			Mobile: req.Mobile,
			Email:  req.Email,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.requestEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if isTimeoutError(err) {
			return nil, ErrGatewayTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrGatewayUnavailable, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 500 {
		if httpResp.StatusCode == http.StatusGatewayTimeout {
			return nil, ErrGatewayTimeout
		}
		return nil, fmt.Errorf("%w: status %d", ErrGatewayUnavailable, httpResp.StatusCode)
	}

	var zpResp zpResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&zpResp); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response", ErrGatewayUnavailable)
	}

	data := parseZpData(zpResp.Data)
	if data == nil || data.Code != 100 {
		code, msg := parseZpError(zpResp.Errors, data)
		return nil, fmt.Errorf("%w: code %d (%s)", ErrGatewayRejected, code, msg)
	}

	authority := data.Authority
	paymentURL := p.startPayBaseURL + authority

	return &PaymentResponse{
		Authority:  authority,
		PaymentURL: paymentURL,
		FeeRials:   data.Fee,
	}, nil
}

type zpVerifyReq struct {
	MerchantID string `json:"merchant_id"`
	Amount     int64  `json:"amount"`
	Authority  string `json:"authority"`
}

func (p *ZarinPalProvider) VerifyPayment(ctx context.Context, req VerifyRequest) (*VerifyResponse, error) {
	if req.AmountRials <= 0 {
		return nil, ErrInvalidAmount
	}
	if req.Authority == "" {
		return nil, errors.New("payments: authority cannot be empty")
	}

	payload := zpVerifyReq{
		MerchantID: p.merchantID,
		Amount:     req.AmountRials,
		Authority:  req.Authority,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.verifyEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if isTimeoutError(err) {
			return nil, ErrGatewayTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrGatewayUnavailable, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 500 {
		if httpResp.StatusCode == http.StatusGatewayTimeout {
			return nil, ErrGatewayTimeout
		}
		return nil, fmt.Errorf("%w: status %d", ErrGatewayUnavailable, httpResp.StatusCode)
	}

	var zpResp zpResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&zpResp); err != nil {
		return nil, fmt.Errorf("%w: failed to decode verify response", ErrGatewayUnavailable)
	}

	data := parseZpData(zpResp.Data)
	// Code 100: Successfully paid and verified
	// Code 101: Already verified (Idempotent verify)
	if data == nil || (data.Code != 100 && data.Code != 101) {
		code, msg := parseZpError(zpResp.Errors, data)
		return nil, fmt.Errorf("%w: verify code %d (%s)", ErrGatewayRejected, code, msg)
	}

	refIDStr := fmt.Sprintf("%v", data.RefID)

	return &VerifyResponse{
		RefID:    refIDStr,
		Code:     data.Code,
		FeeRials: data.Fee,
		CardPan:  data.CardPan,
		CardHash: data.CardHash,
	}, nil
}
