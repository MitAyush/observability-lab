package main

import (
	"log/slog"
	"os"
	"uuid"
)

func main() {
	file, err := os.OpenFile(
		"app.log",
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	logger := slog.New(
		slog.NewJSONHandler(file, nil),
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

func nextService(logger slog.Logger, id string){
	logger.Error(
		"database transaction failed",
		"service", "audit-service",
		"payment_id", "pay-123",
		"correlation_id", id,
	)
}