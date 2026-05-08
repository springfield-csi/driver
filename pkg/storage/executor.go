// Copyright (C) 2023-2026  Red Hat, Inc.
//
// This copyrighted material is made available to anyone wishing to use,
// modify, copy, or redistribute it subject to the terms and conditions of
// the GNU General Public License v.2, or (at your option) any later version.
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY expressed or implied, including the implied warranties of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General
// Public License for more details.  You should have received a copy of the
// GNU General Public License along with this program; if not, write to the
// Free Software Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA
// 02110-1301, USA.  Any Red Hat trademarks that are incorporated in the
// source code or documentation are not subject to the GNU General Public
// License and may only be used or replicated with the express permission of
// Red Hat, Inc.
//
// Red Hat Author(s): Todd Gill <tgill@redhat.com>

package storage

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

// Executor runs commands on the host using nsenter
type Executor struct {
	nsenterPath  string
	hostProcPath string
	useNsenter   bool
}

// NewExecutor creates a new command executor
// If running in a container, set useNsenter=true and ensure /host/proc is mounted
func NewExecutor() *Executor {
	// Auto-detect if we should use nsenter
	useNsenter := false
	if _, err := os.Stat("/host/proc/1/ns/mnt"); err == nil {
		useNsenter = true
		klog.V(4).Info("Detected container environment, will use nsenter")
	} else {
		klog.V(4).Info("Running on host, direct execution")
	}

	return &Executor{
		nsenterPath:  "/usr/bin/nsenter",
		hostProcPath: "/host/proc",
		useNsenter:   useNsenter,
	}
}

// NewExecutorWithNsenter creates an executor with explicit nsenter control
func NewExecutorWithNsenter(useNsenter bool) *Executor {
	return &Executor{
		nsenterPath:  "/usr/bin/nsenter",
		hostProcPath: "/host/proc",
		useNsenter:   useNsenter,
	}
}

// Run executes a command and returns stdout
func (e *Executor) Run(name string, args ...string) (string, error) {
	var cmd *exec.Cmd

	if e.useNsenter {
		// Run in host namespaces
		nsenterArgs := []string{
			fmt.Sprintf("--mount=%s/1/ns/mnt", e.hostProcPath),
			fmt.Sprintf("--ipc=%s/1/ns/ipc", e.hostProcPath),
			"--",
			name,
		}
		nsenterArgs = append(nsenterArgs, args...)
		cmd = exec.Command(e.nsenterPath, nsenterArgs...)
		klog.V(5).Infof("Executing via nsenter: %s %v", name, args)
	} else {
		cmd = exec.Command(name, args...)
		klog.V(5).Infof("Executing: %s %v", name, args)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command '%s %s' failed: %w (stderr: %s)",
			name, strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

// RunWithStderr executes a command and returns both stdout and stderr
func (e *Executor) RunWithStderr(name string, args ...string) (string, string, error) {
	var cmd *exec.Cmd

	if e.useNsenter {
		nsenterArgs := []string{
			fmt.Sprintf("--mount=%s/1/ns/mnt", e.hostProcPath),
			fmt.Sprintf("--ipc=%s/1/ns/ipc", e.hostProcPath),
			"--",
			name,
		}
		nsenterArgs = append(nsenterArgs, args...)
		cmd = exec.Command(e.nsenterPath, nsenterArgs...)
	} else {
		cmd = exec.Command(name, args...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// CommandExists checks if a command is available
func (e *Executor) CommandExists(name string) bool {
	_, err := e.Run("which", name)
	return err == nil
}
