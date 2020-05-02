package spool

import (
	"fmt"
	"os"
	"syscall"
)

// Mostly simple utility functions for working with
// files and locks.

func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

func openLocked(filename string) (*os.File, error) {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		return nil, err
	}
	return file, lockFile(file)
}

func closeLocked(f *os.File) error {
	unlkerr := unlockFile(f)
	if unlkerr != nil {
		return unlkerr
	}
	return f.Close()
}

func openMutex(filename string) (*os.File, error) {
	file, err := openLocked(filename)
	if err != nil {
		return nil, err
	}
	file.WriteString(fmt.Sprintln(os.Getpid()))
	return file, nil
}

func closeMutex(f *os.File) error {
	terr := f.Truncate(0)
	if terr != nil {
		return terr
	}
	return closeLocked(f)
}
