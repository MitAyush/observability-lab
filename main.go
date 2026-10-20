package main

import (
	"log/slog"
	"os"
	"time"
	"uuid"
)

func main() {
	time.Sleep(10* time.Second)
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, nil),
	)

	id := uuid.New().String()
	println(id)

	logger.Info(
		"payment processing started",
		"service", "payment-service",
		"payment_id", "pay-123",
		"correlation_id", id,
	)

	logger.Warn(
		"calling next service",
		"service", "payment-service",
		"payment_id", "pay-123",
		"correlation_id", id,
	)

	nextService(*logger, id)
}

func nextService(logger slog.Logger, id string) {
	logger.Error(
		"database transaction failed",
		"service", "audit-service",
		"payment_id", "pay-123",
		"correlation_id", id,
	)
}