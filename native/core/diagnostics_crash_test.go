package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func init() { crashOutputEnabled = false }

// 后台 goroutine 里未恢复的 panic 会直接结束整个进程（应用闪退）。
// 装上崩溃输出后，panic 信息和调用栈必须落在 <数据目录>/logs/crash.log。
func TestCrashOutputCapturesUnrecoveredGoroutinePanic(t *testing.T) {
	if directory := os.Getenv("GUOAPP_CRASH_CHILD"); directory != "" {
		if err := installCrashOutput(directory); err != nil {
			t.Fatal(err)
		}
		go func() { panic("crash-output-probe") }()
		time.Sleep(5 * time.Second)
		t.Fatal("child should have crashed")
	}
	directory := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestCrashOutputCapturesUnrecoveredGoroutinePanic$", "-test.count=1")
	command.Env = append(os.Environ(), "GUOAPP_CRASH_CHILD="+directory)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("child exited cleanly: %s", output)
	}
	data, readErr := os.ReadFile(crashLogPath(directory))
	if readErr != nil {
		t.Fatalf("crash log missing: %v (child output %s)", readErr, output)
	}
	text := string(data)
	if !strings.Contains(text, "panic: crash-output-probe") || !strings.Contains(text, "goroutine") {
		t.Fatalf("crash log %q", text)
	}
}

func TestRecoveredPanicIsWrittenToCrashLog(t *testing.T) {
	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(crashLogPath(directory)), 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(crashLogPath(directory), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	crashOutput.Lock()
	previous := crashOutput.file
	crashOutput.file = file
	crashOutput.Unlock()
	defer func() {
		crashOutput.Lock()
		crashOutput.file = previous
		crashOutput.Unlock()
		file.Close()
	}()
	recordRecoveredPanic("playback", "boom", []byte("goroutine 1 [running]:\nexample.Frame()"))
	data, _ := os.ReadFile(crashLogPath(directory))
	if !strings.Contains(string(data), `recovered panic in action "playback": boom`) || !strings.Contains(string(data), "example.Frame") {
		t.Fatalf("crash log %q", data)
	}
}
