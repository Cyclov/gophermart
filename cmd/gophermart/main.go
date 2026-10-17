package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Cyclov/gophermart/internal/accrual"
	"github.com/Cyclov/gophermart/internal/auth"
	"github.com/Cyclov/gophermart/internal/config"
	"github.com/Cyclov/gophermart/internal/handler"
	"github.com/Cyclov/gophermart/internal/server"
	"github.com/Cyclov/gophermart/internal/service"
	"github.com/Cyclov/gophermart/internal/storage"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		fmt.Println("Usage: gophermart [-a host:port] [-d connection-string] [-r accrual-url]")
		fmt.Println("Priority: explicit flag > environment > default")
		fmt.Println("-a: RUN_ADDRESS (default localhost:8080)")
		fmt.Println("-d: DATABASE_DSN (required)")
		fmt.Println("JWT_SECRET: required environment variable (at least 32 bytes)")
		fmt.Println("ACCRUAL_WORKERS: parallel accrual requests (1..32, default 4)")
		fmt.Println("-r: ACCRUAL_SYSTEM_ADDRESS (required HTTP or HTTPS URL)")
		return nil
	}
	if err != nil {
		return err
	}
	tokens, err := auth.New(cfg.JWTSecret)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := storage.Connect(ctx, cfg.DBDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("database connected and migrations applied")
	orders := storage.NewOrders(pool)
	worker := accrual.NewWorker(orders, accrual.NewClient(cfg.AccrualURL), logger, cfg.AccrualWorkers)
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); worker.Run(workerCtx) }()
	err = server.Run(ctx, cfg.Addr, handler.NewRouter(service.NewAuth(storage.NewUsers(pool), tokens), tokens, service.NewOrders(orders), service.NewBalances(storage.NewBalances(pool))), logger)
	cancelWorker()
	<-workerDone
	return err
}
