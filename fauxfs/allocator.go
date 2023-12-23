package fauxfs

import (
	"context"
	"fmt"

	"google.golang.org/appengine/log"
)

type FileAllocator interface {
	GetDrive(bytesNeeded int64) (NetDriveStorage, error)
}

type SimpleFileAllocator struct {
	drives []NetDrive
}

func NewSimpleFileAllocator() *SimpleFileAllocator {
	return &SimpleFileAllocator{
		drives: make([]NetDrive, 0),
	}
}

func (s *SimpleFileAllocator) RegisterDrive(drive NetDrive) {
	s.drives = append(s.drives, drive)
}

func (s *SimpleFileAllocator) GetDrive(bytesNeeded int64) (NetDriveStorage, error) {
	for _, drive := range s.drives {
		used, total, err := drive.GetSpace()
		if err != nil {
			log.Errorf(context.Background(), "Failed to retrieve drive metadata: %v", err)
		} else if total-used >= bytesNeeded {
			return drive, nil
		}
	}
	return nil, fmt.Errorf("Failed to find a drive with %d space", bytesNeeded)
}
