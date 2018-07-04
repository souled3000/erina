package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

var (
	pid      int
	progname string

	profCpuFile *os.File
)

func init() {
	pid = os.Getpid()
	paths := strings.Split(os.Args[0], "/")
	paths = strings.Split(paths[len(paths)-1], string(os.PathSeparator))
	progname = paths[len(paths)-1]
}

// BeginProfMem 初始化内存跟踪
func BeginProfMem() {
	runtime.MemProfileRate = 1
}

// EndProfMem 结束内存跟踪，将内存信息写入可执行程序所在的文件夹内，
// 文件为prof/heap_proname-pid-time.prof
func EndProfMem() error {
	runtime.GC()

	dir, err := makeProfDir()
	if err != nil {
		return err
	}
	fn := dir + string(filepath.Separator) +
		fmt.Sprintf("heap_%s_%d_%s.prof", progname, pid, time.Now().Format("2006_01_02_15_04_05"))
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	return pprof.Lookup("heap").WriteTo(f, 1)
}

// BeginProfCPU 开始CPU占用检测
func BeginProfCPU() error {
	dir, err := makeProfDir()
	if err != nil {
		return err
	}
	fn := dir + string(filepath.Separator) +
		fmt.Sprintf("cpu_%s_%d_%s.prof", progname, pid, time.Now().Format("2006_01_02_15_04_05"))
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	err = pprof.StartCPUProfile(f)
	if err != nil {
		f.Close()
		return err
	}
	profCpuFile = f
	return nil
}

// EndProfCPU 结束CPU检测并将信息写入prof文件
func EndProfCPU() error {
	if profCpuFile != nil {
		pprof.StopCPUProfile()
		if err := profCpuFile.Close(); err != nil {
			return err
		}
	}
	return nil
}

func makeProfDir() (string, error) {
	// old
	//dir, err := filepath.Abs(os.Args[0])
	//if err != nil {
	//	return "", err
	//}
	//n := strings.LastIndex(dir, string(filepath.Separator))
	//if n < 0 {
	//	return "", nil
	//}

	dir := GetParentDir(os.Args[0]) + string(filepath.Separator) + "prof"
	err := os.MkdirAll(dir, 0771)
	return dir, err
}

// GetParentDir calculate the parent dir of param path, no matter whether path exists.
func GetParentDir(path string) string {
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	return path[0 : j-1]
}
