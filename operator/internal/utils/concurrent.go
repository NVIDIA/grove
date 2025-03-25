package utils

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

type Task struct {
	Name string
	Fn   func(ctx context.Context) error
}

func RunConcurrently(ctx context.Context, tasks []Task) []error {
	return RunConcurrentlyWithBounds(ctx, tasks, len(tasks))
}

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
