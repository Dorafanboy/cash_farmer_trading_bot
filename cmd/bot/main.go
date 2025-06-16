package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cash-farmer/internal/app"
	"cash-farmer/internal/config"
)

func main() {
	configResult := config.Load()
	configResult.PrintWarnings()

	if configResult.HasErrors() {
		fmt.Fprintf(os.Stderr, "Configuration errors: %s\n", configResult.GetErrorMessage())
		return
	}

	application := app.New(configResult.Config)

	if err := application.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Application initialization error: %v\n", err)
		application.Shutdown()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("Shutdown signal received, gracefully shutting down...")
		cancel()
	}()

	if err := application.RunWithContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Application runtime error: %v\n", err)
	}

	application.Shutdown()
	fmt.Println("Application stopped successfully")
}
