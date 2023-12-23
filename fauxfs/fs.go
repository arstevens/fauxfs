package fauxfs

import (
	"context"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/v2/fs"
)

type fauxFS struct {
	fs.Inode
}

type netFileHandle struct {
	fs.Inode
	open      bool
	mtime     time.Time
	bkFile    *os.File
	bkName    string
	loadFile  func() (*os.File, string, error)
	storeFile func(string) error

	mutex *sync.Mutex
}

func (nf *netFileHandle) openBackingFile() error {
	if nf.open == false {
		var err error
		nf.bkFile, nf.bkName, err = nf.loadFile()
		if err != nil {
			return err
		}
		nf.open = true
	}
	return nil
}

func (nf *netFileHandle) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	nf.mutex.Lock()
	defer nf.mutex.Unlock()

	if err := nf.openBackingFile(); err != nil {
		return nil, 0, 1
	}
	return nf, fuse.FOPEN_DIRECT_IO, 0
}

func (nf *netFileHandle) Flush(ctx context.Context) syscall.Errno {
	nf.mutex.Lock()
	defer nf.mutex.Unlock()

	if nf.open == true {
		if err := nf.bkFile.Close(); err != nil {
			return 1
		}
		if err := nf.storeFile(nf.bkName); err != nil {
			return 1
		}
		nf.open = false
	}
	return 0
}

func (nf *netFileHandle) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	nf.mutex.Lock()
	defer nf.mutex.Unlock()

	if nf.open == false {
		return nil, 1
	}

	n, err := nf.bkFile.ReadAt(dest, off)
	if err != nil {
		return nil, 1
	}
	return fuse.ReadResultData(dest[:n]), 0

}

func (nf *netFileHandle) Write(ctx context.Context, fh fs.FileHandle, buf []byte, off int64) (uint32, syscall.Errno) {
	nf.mutex.Lock()
	defer nf.mutex.Unlock()

	if nf.open == false {
		if err := nf.openBackingFile(); err != nil {
			return 0, 1
		}
	}

	info, err := nf.bkFile.Stat()
	if err != nil {
		return 0, 1
	}

	if off > info.Size() {
		return 0, 1
	}

	n, err := nf.bkFile.WriteAt(buf, off)
	if err != nil {
		return uint32(n), 1
	}
	return uint32(n), 0
}

func (root *fauxFS) OnAdd(ctx context.Context) {

}
