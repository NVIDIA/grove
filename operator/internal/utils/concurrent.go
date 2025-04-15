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

package utils

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// Task is a named closure.
type Task struct {
	Name string
	Fn   func(ctx context.Context) error
}

// RunConcurrently executes a slice of Tasks concurrently.
func RunConcurrently(ctx context.Context, tasks []Task) []error {
	return RunConcurrentlyWithBounds(ctx, tasks, len(tasks))
}

// RunConcurrentlyWithBounds executes a slice of Tasks with at most `bound` tasks running concurrently.
func RunConcurrentlyWithBounds(ctx context.Context, tasks []Task, bound int) []error {
	rg := newRunGroup(bound)
	for _, task := range tasks {
		rg.trigger(ctx, task)
	}
	return rg.waitAndCollectErrors()
}

type runGroup struct {
	wg    sync.WaitGroup
	errCh chan error
}

func newRunGroup(numTasks int) *runGroup {
	return &runGroup{
		wg:    sync.WaitGroup{},
		errCh: make(chan error, numTasks),
	}
}

func (rg *runGroup) trigger(ctx context.Context, task Task) {
	rg.wg.Add(1)
	go func(task Task) {
		defer rg.wg.Done()
		defer func() {
			if v := recover(); v != nil {
				stack := debug.Stack()
				panicErr := fmt.Errorf("task: %s execution panicked: %v\n, stack-trace: %s", task.Name, v, stack)
				rg.errCh <- panicErr
			}
		}()
		if err := task.Fn(ctx); err != nil {
			rg.errCh <- err
		}
	}(task)
}

func (rg *runGroup) waitAndCollectErrors() []error {
	rg.wg.Wait()
	close(rg.errCh)
	var errs []error
	for err := range rg.errCh {
		errs = append(errs, err)
	}
	return errs
}
