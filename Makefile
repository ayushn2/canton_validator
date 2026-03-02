# Makefile for canton_validator project

# Go command
GO=go run

# Default file paths
WALLET_CMD=./cmd/wallet/main.go
TRANSFER_CMD=./cmd/transfer/main.go
LEDGER_CMD=./cmd/ledger/main.go

.PHONY: wallet transfer ledger test clean help

# ------------------------------
# Wallet setup
# ------------------------------
wallet:
	@$(GO) $(WALLET_CMD)

# ------------------------------
# Execute a transfer
# ------------------------------
transfer:
	@$(GO) $(TRANSFER_CMD)

# ------------------------------
# Run ledger commands
# ------------------------------
ledger:
	@$(GO) $(LEDGER_CMD)

# ------------------------------
# Run all: wallet + transfer
# ------------------------------
all: wallet transfer

# ------------------------------
# Run tests
# ------------------------------
test:
	@echo "🧪 Running Go tests..."
	go test ./...

# ------------------------------
# Clean build artifacts
# ------------------------------
clean:
	@echo "🧹 Cleaning Go build cache..."
	go clean -cache -testcache
	@echo "Done."

# ------------------------------
# Show help
# ------------------------------
help:
	@echo "Available commands:"
	@echo "  make wallet                  # Setup a wallet"
	@echo "  make transfer                # Execute a transfer"
	@echo "  make ledger                  # Run ledger commands"
	@echo "  make all                     # Run wallet setup and then transfer"
	@echo "  make test                    # Run all Go tests"
	@echo "  make clean                   # Clean build/test cache"