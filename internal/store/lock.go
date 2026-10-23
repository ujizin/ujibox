package store

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Lock is an flock on a file. The kernel drops it when the process dies, so a
// crash never leaves a stale lock.
type Lock struct{ f *os.File }

// ErrLocked means another process holds the lock.
var ErrLocked = errors.New("locked by another process")

func openLock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
}

// LockExclusive waits for an exclusive lock.
func LockExclusive(path string) (*Lock, error) {
	return lock(path, unix.LOCK_EX)
}

// TryLockExclusive takes an exclusive lock or returns ErrLocked.
func TryLockExclusive(path string) (*Lock, error) {
	return lock(path, unix.LOCK_EX|unix.LOCK_NB)
}

// TryLockShared takes a shared lock or returns ErrLocked.
func TryLockShared(path string) (*Lock, error) {
	return lock(path, unix.LOCK_SH|unix.LOCK_NB)
}

func lock(path string, how int) (*Lock, error) {
	f, err := openLock(path)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return &Lock{f: f}, nil
}

// Unlock releases the lock. It is safe on a nil lock.
func (l *Lock) Unlock() {
	if l == nil || l.f == nil {
		return
	}
	_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	_ = l.f.Close()
	l.f = nil
}
