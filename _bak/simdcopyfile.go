package cmd

import (
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/cpu"
)

var (
	isAVX512       bool
	isAVX2         bool
	isSSE3         bool
	isASIMD        bool
	uintptrAlign   uintptr
	uintptrBufSize uintptr
)

func init() {
	if cpu.X86.HasAVX512 {
		isAVX512 = true
	}

	if cpu.X86.HasAVX2 {
		isAVX2 = true
	}

	if cpu.X86.HasSSE3 {
		isSSE3 = true
	}

	if cpu.ARM64.HasASIMD {
		isASIMD = true
	}

	uintptrAlign = uintptr(64)
	uintptrBufSize = uintptr(bufSize)
}

func simdCopyFile(src, dst string, finfo os.FileInfo) (writeSize int64, err error) {
	srcFd, err := syscall.Open(src, syscall.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}

	dstTemp := strings.Join([]string{dst, "ing"}, ".")
	dstFd, err := syscall.Open(dstTemp, syscall.O_CREAT|syscall.O_WRONLY|syscall.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}

	buf := make([]byte, bufSize)
	alignedBuf := alignBuffer(buf)

	for {
		n, err := syscall.Read(srcFd, alignedBuf)
		if n == 0 || err != nil {
			break
		}

		simdCopy(alignedBuf[:n])

		_, err = syscall.Write(dstFd, alignedBuf[:n])
		if err != nil {
			return 0, err
		}
	}

	syscall.Close(dstFd)
	syscall.Close(srcFd)

	if err := os.Rename(dstTemp, dst); err != nil {
		return 0, err
	}

	if err := chmodFile(dst, finfo); err != nil {
		return 0, err
	}

	return finfo.Size(), nil
}

func alignBuffer(buf []byte) []byte {
	offset := (uintptrAlign - (uintptr(unsafe.Pointer(&buf[0])) % uintptrAlign)) % uintptrAlign
	return buf[offset : offset+uintptrBufSize]
}

func simdCopy(data []byte) {
	switch {
	case isASIMD:
		neonCopy(data)
	case isAVX512:
		avx512Copy(data)
	case isAVX2:
		avx2Copy(data)
	case isSSE3:
		sseCopy(data)
	}
}

func avx512Copy(data []byte) {
}

func avx2Copy(data []byte) {
}

func sseCopy(data []byte) {
}

func neonCopy(data []byte) {
}

func getCPUFlags() string {
	cfs := []string{}
	if cpu.X86.HasAVX512 {
		cfs = append(cfs, "avx512")
	}

	if cpu.X86.HasAVX2 {
		cfs = append(cfs, "avx2")
	}

	if cpu.X86.HasSSE3 {
		cfs = append(cfs, "sse3")
	}

	if cpu.ARM64.HasASIMD {
		cfs = append(cfs, "asimd")
	}

	if len(cfs) == 0 {
		return ""
	}

	return strings.Join(cfs, " ")
}

func chmodFile(dst string, finfo os.FileInfo) (err error) {
	err = os.Chmod(dst, finfo.Mode())
	if err != nil {
		PrintError("chmodFile: os.Chmod", err)
		return err
	}

	err = os.Chtimes(dst, finfo.ModTime(), finfo.ModTime())
	if err != nil {
		PrintError("chmodFile: os.Chtimes", err)
		return err
	}

	return nil
}
