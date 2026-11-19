.PHONY: verify verify-psychology verify-go verify-ts

verify: verify-psychology

verify-psychology: verify-go verify-ts
	@echo "All psychology platform checks passed successfully!"

verify-go:
	cd apps/psychology-api && go vet ./... && go test -v ./...

verify-ts:
	pnpm typecheck
	pnpm test
	pnpm build
