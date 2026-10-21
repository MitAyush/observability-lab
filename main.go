package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

func main() {
	time.Sleep(10 * time.Second)
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, nil),
	)

	for id := 1; id <= 500; id++ {
		doLog(fmt.Sprintf("exp-11-%d", id), logger)
		time.Sleep(50 * time.Millisecond)
	}

}

func doLog(id string, logger *slog.Logger) {
	logger.Info(
		"payment processing started",
		"service", "payment-service",
		"payment_id", "pay-123",
		"correlation_id", id,
	)

	// logger.Warn(
	// 	"calling next service",
	// 	"service", "payment-service",
	// 	"payment_id", "pay-123",
	// 	"correlation_id", id,
	// )

	// nextService(logger, id)
}

func nextService(logger *slog.Logger, id string) {
	logger.Error(
		"database transaction failed",
		"service", "audit-service",
		"payment_id", "pay-123",
		"correlation_id", id,
	)
}
