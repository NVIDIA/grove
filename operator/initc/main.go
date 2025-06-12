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

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

var l logr.Logger

func main() {
	ctx := setupSignalHandler()

	// TODO: @renormalize fix the logging
	l1, err := zap.NewDevelopment()
	if err != nil {
		panic("logging err")
	}
	l = zapr.NewLogger(l1).WithName("grove-initc")

	l.Info("Starting grove init container", "version", getVersion())

	config, err := initializeConfig()
	if err != nil {
		l.Error(err, "failed to generate configuration for the init container from the flags")
		os.Exit(1)
	}

	printVersionAndExitIfRequested()

	if err := run(ctx, config); err != nil {
		l.Error(err, "failed to wait for all PodCliques")
		os.Exit(1)
	}

	l.Info("Successfully waited for all dependent PodCliques to start up")
}

// setupSignalHandler sets up the context for the application. Handles the SIGTERM and SIGINT signals.
// The returned context gets cancelled by the first signal. The second signal causes the program with exit code 1.
func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ch
		cancel()
		<-ch
		os.Exit(1)
	}()

	return ctx
}
