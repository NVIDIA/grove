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
	"log/slog"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/rand"
)

type taskType int

const (
	successful taskType = iota
	panicky
	erroneous
)

type testTaskConfig struct {
	namePrefix string
	numTasks   int
	taskType   taskType
}

func TestRunConcurrently(t *testing.T) {
	testCases := []struct {
		description             string
		taskConfigs             []testTaskConfig
		expectedSuccessfulTasks []string
		expectedFailedTasks     []string
	}{
		{
			description: "All taskConfigs succeed",
			taskConfigs: []testTaskConfig{
				{namePrefix: "task-a", numTasks: 1, taskType: successful},
				{namePrefix: "task-b", numTasks: 1, taskType: successful},
				{namePrefix: "task-c", numTasks: 1, taskType: successful},
			},
			expectedSuccessfulTasks: []string{"task-a-0", "task-b-0", "task-c-0"},
			expectedFailedTasks:     []string{},
		},
		{
			description: "One task panics, others succeed",
			taskConfigs: []testTaskConfig{
				{namePrefix: "task-a", numTasks: 2, taskType: successful},
				{namePrefix: "task-b", numTasks: 1, taskType: panicky},
			},
			expectedSuccessfulTasks: []string{"task-a-0", "task-a-1"},
			expectedFailedTasks:     []string{"task-b-0"},
		},
		{
			description: "One task panics, one errors out and one succeeds",
			taskConfigs: []testTaskConfig{
				{namePrefix: "task-a", numTasks: 1, taskType: successful},
				{namePrefix: "task-b", numTasks: 1, taskType: panicky},
				{namePrefix: "task-c", numTasks: 1, taskType: erroneous},
			},
			expectedSuccessfulTasks: []string{"task-a-0"},
			expectedFailedTasks:     []string{"task-b-0", "task-c-0"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actualRunResult := RunConcurrently(context.Background(), logr.Discard(), createTasks(tc.taskConfigs))
			assert.Equal(t, tc.expectedSuccessfulTasks, actualRunResult.SuccessfulTasks)
			assert.Equal(t, tc.expectedFailedTasks, actualRunResult.FailedTasks)
		})
	}
}

func TestRunConcurrentlyWithSlowStart(t *testing.T) {
	testCases := []struct {
		description             string
		taskConfigs             []testTaskConfig
		initialBatchSize        int
		expectedSuccessfulTasks int
		expectedFailedTasks     int
		expectedSkippedTasks    int
	}{
		{
			description: "All taskConfigs succeed",
			taskConfigs: []testTaskConfig{
				{namePrefix: "task-a", numTasks: 3, taskType: successful},
			},
			initialBatchSize:        1,
			expectedSuccessfulTasks: 3,
		},
		{
			description: "One task panics, some succeed and others are skipped",
			taskConfigs: []testTaskConfig{
				{namePrefix: "task-a", numTasks: 1, taskType: successful},
				{namePrefix: "task-b", numTasks: 1, taskType: panicky},
				{namePrefix: "task-c", numTasks: 4, taskType: successful},
			},
			initialBatchSize:        1,
			expectedSuccessfulTasks: 2,
			expectedFailedTasks:     1,
			expectedSkippedTasks:    3,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tasks := createTasks(tc.taskConfigs)
			actualRunResult := RunConcurrentlyWithSlowStart(context.Background(), logr.Discard(), tc.initialBatchSize, tasks)

			assert.Equal(t, tc.expectedSuccessfulTasks, len(actualRunResult.SuccessfulTasks))
			assert.Equal(t, tc.expectedFailedTasks, len(actualRunResult.FailedTasks))
			assert.Equal(t, tc.expectedSkippedTasks, len(actualRunResult.SkippedTasks))
		})
	}
}

func createTasks(taskConfigs []testTaskConfig) []Task {
	var tasks []Task
	for _, taskConfig := range taskConfigs {
		tasks = append(tasks, doCreateTasks(taskConfig.taskType, taskConfig.namePrefix, taskConfig.numTasks)...)
	}
	return tasks
}

func doCreateTasks(taskType taskType, namePrefix string, num int) []Task {
	tasks := make([]Task, 0, num)
	for i := 0; i < num; i++ {
		name := fmt.Sprintf("%s-%d", namePrefix, i)
		task := Task{Name: name}
		switch taskType {
		case successful:
			task.Fn = createSuccessfulTaskFn(name)
		case panicky:
			task.Fn = createPanickyTaskFn(name)
		case erroneous:
			task.Fn = createErringTaskFn(name)
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func createSuccessfulTaskFn(name string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		tick := time.NewTicker(generateRandomDelay())
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-tick.C:
				slog.Info("Task completed successfully", "taskName", name)
				return nil
			}
		}
	}
}

func createPanickyTaskFn(name string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		tick := time.NewTicker(generateRandomDelay())
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-tick.C:
				panic(fmt.Sprintf("task %s panicked", name))
			}
		}
	}
}

func createErringTaskFn(name string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		tick := time.NewTicker(generateRandomDelay())
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-tick.C:
				return fmt.Errorf("task %s encountered an error", name)
			}
		}
	}
}

func generateRandomDelay() time.Duration {
	return time.Duration(5+rand.Intn(15)) * time.Millisecond
}
