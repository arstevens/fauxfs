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
	bkFile    *os.File
	bkName    string
	mutex *sync.Mutex

	loadFile  func() (*os.File, string, error)
	storeFile func(io.Reader) error
}

func newNetFileHandle(internalFname string, drive NetDriveStorage) (*netFileHandle, error) {
	tmpFile, err := os.CreateTemp("", "faux.*.file") 
	if err != nil {
		return nil, err
	}
	bkName := tmpFile.Name()
	tmpFile.Close()

	return &netFileHandle{
		open: false,
		bkFile: nil,
		bkName: "",
		mutex: &sync.Mutex{},
		loadFile: func() (*os.File, string, error) {
			file, err := os.OpenFile(bkName, os.O_RDWR|os.O_CREATE, 644)
			if err != nil {
				return nil, "", err
			}
			defer file.Close()

			if err = drive.Download(internalFname, file); err != nil {
				return nil, "", err
			}
			if _, err = file.Seek(0, 0); err != nil {
				return nil, "", err
			}
			return file, bkName, nil 
		},
		storeFile: func(in io.Reader) (string, error) {
			if err := drive.Delete(internalFname); err != nil {
				return "", err
			}

			fileID, err := drive.Upload(in)
			if err != nil {
				return "", err
			}
			return fileID, nil
		},
	}, nil
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
		if _, err := nf.bkFile.Seek(0, 0); err != nil {
			return 1
		}
		if err := nf.storeFile(nf.bkFile); err != nil {
			return 1
		}
		nf.bkFile.Close()
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

type fauxDirHandle struct {
	fs.Inode
	dirEntries map[string]fuse.DirEntry
}

func (dh *fauxDirHandle) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewListDirStream(dh.dirEntries), 0
}

func (dh *fauxDirHandle) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	for entryName, entry := range dh.dirEntries {
		if entryName == name {
			var child *fs.Inode 
			if entry.Mode == fuse.S_IFREG {

			} else entry.Mode == fuse.S_IFDIR {

			} else {

			}
		}
	}
}
