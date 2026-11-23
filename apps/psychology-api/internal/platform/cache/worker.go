package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type CDNPurger interface {
	Purge(ctx context.Context, host string, paths []string) error
}

// ProxyPurger connects an in-memory EdgeCacheProxy to the CDNPurger interface.
type ProxyPurger struct {
	proxy *EdgeCacheProxy
}

func NewProxyPurger(proxy *EdgeCacheProxy) *ProxyPurger {
	return &ProxyPurger{proxy: proxy}
}

func (p *ProxyPurger) Purge(ctx context.Context, host string, paths []string) error {
	p.proxy.Purge(host, paths)
	return nil
}

// ArvanCloudPurger sends cache invalidation requests to ArvanCloud's API for the tenant's domain.
type ArvanCloudPurger struct {
	apiKey     string
	httpClient *http.Client
	apiBaseURL string
}

func NewArvanCloudPurger(apiKey string, client *http.Client) *ArvanCloudPurger {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &ArvanCloudPurger{
		apiKey:     apiKey,
		httpClient: client,
		apiBaseURL: "https://napi.arvancloud.ir/cdn/4.0",
	}
}

func (a *ArvanCloudPurger) Purge(ctx context.Context, host string, paths []string) error {
	if a.apiKey == "" {
		// In development or test environments where API key is not configured, simulate successful purge
		return nil
	}

	urls := make([]string, 0, len(paths))
	for _, p := range paths {
		urls = append(urls, CanonicalURL(host, p))
	}

	payload := map[string]any{
		"purge": "individual",
		"urls":  urls,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal arvancloud purge request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/domains/%s/caching/purge", a.apiBaseURL, host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create arvancloud purge request: %w", err)
	}

	req.Header.Set("Authorization", a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("arvancloud purge request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("arvancloud purge returned error status %d", resp.StatusCode)
	}

	return nil
}

// OutboxWorker processes pending outbox events asynchronously.
type OutboxWorker struct {
	db         OutboxDBTX
	purger     CDNPurger
	maxRetries int
}

func NewOutboxWorker(db OutboxDBTX, purger CDNPurger) *OutboxWorker {
	return &OutboxWorker{
		db:         db,
		purger:     purger,
		maxRetries: 3,
	}
}

// ProcessBatch reads up to batchSize pending events and executes cache purges.
// Returns the number of successfully processed events and any fatal database error.
func (w *OutboxWorker) ProcessBatch(ctx context.Context, batchSize int) (int, error) {
	events, err := FetchPendingEvents(ctx, w.db, batchSize)
	if err != nil {
		return 0, err
	}

	successCount := 0
	for _, event := range events {
		host := event.Payload.Host
		paths := event.Payload.Paths

		purgeErr := w.purger.Purge(ctx, host, paths)
		if purgeErr != nil {
			newRetry := event.RetryCount + 1
			_ = MarkEventFailed(ctx, w.db, event.ID, purgeErr.Error(), newRetry, w.maxRetries)
			continue
		}

		if err := MarkEventCompleted(ctx, w.db, event.ID); err != nil {
			return successCount, err
		}
		successCount++
	}

	return successCount, nil
}

// Run starts a background loop that processes pending outbox events every interval until ctx is cancelled.
func (w *OutboxWorker) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				n, err := w.ProcessBatch(ctx, 50)
				if err != nil || n == 0 {
					break
				}
			}
		}
	}
}
