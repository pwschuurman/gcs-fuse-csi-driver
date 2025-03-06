/*
Copyright 2018 The Kubernetes Authors.
Copyright 2024 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"k8s.io/klog/v2"
)

const (
	mountPathsLocation = "/volumes/"
)

var (
	supportedMachineRegex = regexp.MustCompile("^projects/[0-9]+/machineTypes/(a3|a4|ct5l|ct5lp|ct5p|ct6e)-.*$")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// Create cancellable context to pass into exec.
	ctx, cancel := context.WithCancel(context.Background())

	// Handle SIGTERM signal.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	go func() {
		<-sigs
		klog.Info("Caught SIGTERM signal: Terminating...")
		cancel()

		os.Exit(0) // Exit gracefully
	}()

	machineType, err := getMachineType(ctx)
	if err != nil {
		klog.Warningf("Unable to fetch machine-type, ignoring: %v", err)
	}

	val := os.Getenv("USER_ENABLED_METADATA_PREFETCH")
	enablePrefetch := true
	if val == "TRUE" {
		enablePrefetch = true
	} else if val == "FALSE" {
		enablePrefetch = false
	} else {
		// Defaulting scenario, env var unset.
		enablePrefetch = supportedMachineRegex.MatchString(machineType)
	}

	if enablePrefetch {
		// Start the "ls" command in the background.
		// All our volumes are mounted under the /volumes/ directory.
		cmd := exec.CommandContext(ctx, "ls", "-R", mountPathsLocation)
		cmd.Stdout = nil // Connects file descriptor to the null device (os.DevNull).

		// TODO(hime): We should research stratergies to parallelize ls execution and speed up cache population.
		err := cmd.Start()
		if err == nil {
			mountPaths, err := getDirectoryNames(mountPathsLocation)
			if err == nil {
				klog.Infof("Running ls on mountPath(s): %s", strings.Join(mountPaths, ", "))
			} else {
				klog.Warningf("failed to get mountPaths: %v", err)
			}

			err = cmd.Wait()
			if err != nil {
				klog.Errorf("Error while executing ls command: %v", err)
			} else {
				klog.Info("Metadata prefetch complete")
			}
		} else {
			klog.Errorf("Error starting ls command: %v.", err)
		}
	}

	klog.Info("Going to sleep...")

	// Keep the process running.
	select {}
}

func getMachineType(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://metadata.google.internal/computeMetadata/v1/instance/machine-type", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get machine type: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// getDirectoryNames returns a list of strings representing the names of
// the directories within the provided path.
func getDirectoryNames(dirPath string) ([]string, error) {
	directories := []string{}
	items, err := os.ReadDir(dirPath)
	if err != nil {
		return directories, err
	}

	for _, item := range items {
		if item.IsDir() {
			directories = append(directories, item.Name())
		}
	}

	return directories, nil
}
