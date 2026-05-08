// Copyright (C) 2023-2026  Red Hat, Inc.
//
// Mock executor for testing storage managers

package testutil

import (
	"fmt"
	"strings"
	"sync"
)

// MockExecutor is a mock implementation of the Executor interface for testing
type MockExecutor struct {
	mu        sync.Mutex
	commands  []Command
	responses map[string]Response
}

// Command represents a command that was executed
type Command struct {
	Name string
	Args []string
}

// Response represents a command response
type Response struct {
	Stdout string
	Stderr string
	Err    error
}

// NewMockExecutor creates a new mock executor
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		commands:  make([]Command, 0),
		responses: make(map[string]Response),
	}
}

// AddResponse adds a response for a specific command
func (m *MockExecutor) AddResponse(cmdName string, resp Response) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[cmdName] = resp
}

// AddResponseForArgs adds a response for a command with specific arguments
func (m *MockExecutor) AddResponseForArgs(cmdName string, args []string, resp Response) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s %s", cmdName, strings.Join(args, " "))
	m.responses[key] = resp
}

// Run executes a command (mock implementation)
func (m *MockExecutor) Run(name string, args ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the command
	m.commands = append(m.commands, Command{
		Name: name,
		Args: args,
	})

	// Try exact match first (command + args)
	key := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	if resp, ok := m.responses[key]; ok {
		return resp.Stdout, resp.Err
	}

	// Try command name only
	if resp, ok := m.responses[name]; ok {
		return resp.Stdout, resp.Err
	}

	// Default: return success with empty output
	return "", nil
}

// RunWithStderr executes a command and returns both stdout and stderr
func (m *MockExecutor) RunWithStderr(name string, args ...string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the command
	m.commands = append(m.commands, Command{
		Name: name,
		Args: args,
	})

	// Try exact match
	key := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	if resp, ok := m.responses[key]; ok {
		return resp.Stdout, resp.Stderr, resp.Err
	}

	// Try command name only
	if resp, ok := m.responses[name]; ok {
		return resp.Stdout, resp.Stderr, resp.Err
	}

	return "", "", nil
}

// CommandExists checks if a command is available (always returns true in mock)
func (m *MockExecutor) CommandExists(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if there's a response configured for this command
	if _, ok := m.responses[name]; ok {
		return true
	}

	// Default: assume command exists
	return true
}

// GetCommands returns all commands that were executed
func (m *MockExecutor) GetCommands() []Command {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid race conditions
	return append([]Command{}, m.commands...)
}

// GetLastCommand returns the last command that was executed
func (m *MockExecutor) GetLastCommand() *Command {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.commands) == 0 {
		return nil
	}

	cmd := m.commands[len(m.commands)-1]
	return &cmd
}

// Reset clears all recorded commands
func (m *MockExecutor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands = make([]Command, 0)
}

// AssertCommandCalled checks if a command was called with specific arguments
func (m *MockExecutor) AssertCommandCalled(name string, expectedArgs ...string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cmd := range m.commands {
		if cmd.Name != name {
			continue
		}

		if len(cmd.Args) != len(expectedArgs) {
			continue
		}

		match := true
		for i, arg := range expectedArgs {
			if cmd.Args[i] != arg {
				match = false
				break
			}
		}

		if match {
			return true
		}
	}

	return false
}

// CommandCallCount returns the number of times a command was called
func (m *MockExecutor) CommandCallCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, cmd := range m.commands {
		if cmd.Name == name {
			count++
		}
	}

	return count
}
