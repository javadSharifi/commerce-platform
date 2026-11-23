.PHONY: verify verify-psychology verify-go verify-ts test-db test-browser test-fixture test-fixture-live

verify: verify-psychology

verify-psychology: verify-go verify-ts test-fixture
	@echo "All psychology platform checks passed successfully!"

verify-go:
	cd apps/psychology-api && go vet ./... && go test -v ./...

test-db:
	node scripts/test-db.mjs

test-browser:
	pnpm build
	PSYCHOLOGY_TEST_BROWSER=1 node scripts/test-db.mjs

test-fixture:
	node --test scripts/test-phase1.test.mjs

test-fixture-live:
	node --test scripts/test-phase1-live.test.mjs

verify-ts:
	pnpm build
	pnpm typecheck
	pnpm test
