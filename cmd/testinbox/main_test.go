package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestProcessSignals(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("SIGINT/SIGTERM process signaling is not supported on this OS")
	}
	binary := filepath.Join(t.TempDir(), "testinbox")
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), time.Minute)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, ".")
	build.WaitDelay = time.Second
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build application: %v\n%s", err, output)
	}

	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := listener.Addr().String()
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}

			outputPath := filepath.Join(t.TempDir(), "process.log")
			output, err := os.Create(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = output.Close() })
			processCtx, cancelProcess := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancelProcess()
			command := exec.CommandContext(processCtx, binary)
			command.Env = append(os.Environ(),
				"TESTINBOX_HTTP_ADDR="+addr,
				"TESTINBOX_SMTP_ADDR=127.0.0.1:1025",
				"TESTINBOX_DB_PATH="+filepath.Join(t.TempDir(), "unused.db"),
			)
			command.Stdout, command.Stderr = output, output
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			var waitErr error
			go func() { waitErr = command.Wait(); close(done) }()
			t.Cleanup(func() {
				_ = command.Process.Kill()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("subprocess did not exit after kill")
				}
				if t.Failed() {
					logs, err := os.ReadFile(outputPath)
					if err != nil {
						t.Logf("read process output: %v", err)
					} else {
						t.Logf("process output:\n%s", logs)
					}
				}
			})

			client := &http.Client{Timeout: 250 * time.Millisecond, Transport: &http.Transport{DisableKeepAlives: true}}
			defer client.CloseIdleConnections()
			startupCtx, cancelStartup := context.WithTimeout(processCtx, 10*time.Second)
			defer cancelStartup()
			poll := time.NewTicker(20 * time.Millisecond)
			defer poll.Stop()
			var lastErr error
			var lastStatus int
			for {
				request, err := http.NewRequestWithContext(startupCtx, http.MethodGet, "http://"+addr+"/health/live", nil)
				if err != nil {
					t.Fatal(err)
				}
				response, err := client.Do(request)
				lastErr = err
				if err == nil {
					lastStatus = response.StatusCode
					_ = response.Body.Close()
					if lastStatus == http.StatusOK {
						break
					}
				}
				select {
				case <-done:
					t.Fatalf("application exited before liveness: %v", waitErr)
				case <-startupCtx.Done():
					t.Fatalf("liveness deadline: %v (last status %d, error %v)", startupCtx.Err(), lastStatus, lastErr)
				case <-poll.C:
				}
			}

			if err := command.Process.Signal(signal); err != nil {
				t.Fatalf("send %s: %v", signal, err)
			}
			select {
			case <-done:
				if waitErr != nil {
					t.Fatalf("unclean exit after %s: %v", signal, waitErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("application did not exit after %s", signal)
			}
			connection, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
			if err == nil {
				_ = connection.Close()
				t.Fatal("HTTP listener still accepts connections after process exit")
			}
		})
	}
}
