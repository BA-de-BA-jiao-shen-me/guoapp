package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

// crashLogBytes 超过这个大小时，启动时把 crash.log 轮换成 crash.log.1。
const crashLogBytes = 1 << 20

// crashOutputEnabled 在单元测试里关掉（见 diagnostics_crash_test.go），
// 免得进程级的崩溃输出指向某个用例的临时目录。
var crashOutputEnabled = true

var crashOutput struct {
	sync.Mutex
	file *os.File
}

// crashLogPath 是 Go 核心崩溃日志的位置：<应用数据目录>/logs/crash.log。
// Windows 上应用数据目录是 %APPDATA%\短剧集合\短剧集合。
func crashLogPath(directory string) string {
	return filepath.Join(directory, "logs", "crash.log")
}

// installCrashOutput 让 Go 运行时把致命错误（后台 goroutine 里未恢复的 panic、
// concurrent map writes 等）连同调用栈写进 crash.log。
// 核心以 DLL/so 形式嵌在应用里，这类错误会直接结束整个进程（用户看到的就是闪退），
// 而 GUI 程序的 stderr 没有地方显示，不写文件就完全没有线索。
func installCrashOutput(directory string) error {
	crashOutput.Lock()
	defer crashOutput.Unlock()
	if crashOutput.file != nil {
		return nil
	}
	path := crashLogPath(directory)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil && info.Size() > crashLogBytes {
		_ = os.Rename(path, path+".1")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if err := debug.SetCrashOutput(file, debug.CrashOptions{}); err != nil {
		file.Close()
		return err
	}
	crashOutput.file = file
	return nil
}

// recordRecoveredPanic 把被 NativeRequest 拦下的 panic 和调用栈记进 crash.log，
// 这类 panic 不会闪退，但会让那一次请求失败，留下调用栈才能定位。
func recordRecoveredPanic(action string, value any, stack []byte) {
	crashOutput.Lock()
	defer crashOutput.Unlock()
	if crashOutput.file == nil {
		return
	}
	_, _ = fmt.Fprintf(crashOutput.file, "\n=== %s recovered panic in action %q: %v\n%s\n", time.Now().Format(time.RFC3339), action, value, truncate(string(stack), 16*1024))
}
