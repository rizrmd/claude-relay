// Example of running multiple isolated Claude instances.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rizrmd/claude-relay"
)

func main() {
	// Create multiple isolated Claude instances
	instances := []struct {
		name    string
		port    string
		baseDir string
	}{
		{"Instance 1", "8081", "./claude-1"},
		{"Instance 2", "8082", "./claude-2"},
		{"Instance 3", "8083", "./claude-3"},
	}

	var relays []*clauderelay.Relay
	var wg sync.WaitGroup

	// Start all instances
	for _, inst := range instances {
		relay, err := clauderelay.New(clauderelay.Options{
			Port:          inst.port,
			BaseDir:       inst.baseDir,
			AutoSetup:     true,
			EnableLogging: false, // Reduce noise with multiple instances
		})
		if err != nil {
			log.Printf("Failed to create %s: %v", inst.name, err)
			continue
		}

		if err := relay.Start(); err != nil {
			log.Printf("Failed to start %s: %v", inst.name, err)
			relay.Close()
			continue
		}

		fmt.Printf("%s started on %s\n", inst.name, relay.GetWebSocketURL())
		relays = append(relays, relay)
	}

	if len(relays) == 0 {
		log.Fatal("No instances started successfully")
	}

	fmt.Println("\nAll instances running. Press Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down all instances...")

	// Stop all instances concurrently
	for i, relay := range relays {
		wg.Add(1)
		go func(r *clauderelay.Relay, name string) {
			defer wg.Done()
			if err := r.Close(); err != nil {
				log.Printf("Error stopping %s: %v", name, err)
			} else {
				fmt.Printf("%s stopped\n", name)
			}
		}(relay, instances[i].name)
	}

	wg.Wait()
	fmt.Println("All instances stopped")
}