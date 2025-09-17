// /*
// Copyright 2025 The Grove Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"github.com/NVIDIA/grove/operator/initc/cmd/opts"
	"github.com/NVIDIA/grove/operator/initc/internal"
	"github.com/NVIDIA/grove/operator/internal/logger"
	"github.com/NVIDIA/grove/operator/internal/version"
)

// log is the global logger instance configured for the grove init container.
// It uses JSON format at INFO level for structured logging.
var (
	log = logger.MustNewLogger(false, configv1alpha1.InfoLevel, configv1alpha1.LogFormatJSON).WithName("grove-initc")
)

// main is the entry point for the grove init container.
// It parses CLI options, sets up signal handling, and waits for parent PodCliques to be ready.
func main() {
	// Set up signal handling context for graceful shutdown
	ctx := setupSignalHandler()

	// Parse command line options and configuration
	config, err := opts.InitializeCLIOptions()
	if err != nil {
		log.Error(err, "Failed to generate configuration for the init container from the flags")
		os.Exit(1)
	}
	version.PrintVersionAndExitIfRequested()

	log.Info("Starting grove init container", "version", version.Get())

	// Extract PodClique dependencies from parsed configuration
	podCliqueDependencies, err := config.GetPodCliqueDependencies()
	if err != nil {
		log.Error(err, "Failed to parse CLI input")
		os.Exit(1)
	}

	// Initialize the state tracker for parent PodCliques
	podCliqueState, err := internal.NewPodCliqueState(podCliqueDependencies, log)
	if err != nil {
		os.Exit(1)
	}

	// Wait for all parent PodCliques to reach the ready state
	if err = podCliqueState.WaitForReady(ctx, log); err != nil {
		log.Error(err, "Failed to wait for all parent PodCliques")
		os.Exit(1)
	}

	log.Info("Successfully waited for all parent PodCliques to start up")
}

// setupSignalHandler sets up the context for the application and handles SIGTERM and SIGINT signals.
// The returned context gets cancelled by the first signal. The second signal causes the program to exit with code 1.
func setupSignalHandler() context.Context {
	// Create a cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Set up signal channel with buffer for two signals
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	// Handle signals in a goroutine
	go func() {
		// First signal triggers graceful shutdown
		<-ch
		cancel()
		// Second signal forces immediate exit
		<-ch
		os.Exit(1)
	}()

	return ctx
}
