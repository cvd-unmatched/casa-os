//go:build !windows

package service

import (
	"archive/tar"
	"fmt"
	"os"
	"syscall"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"go.uber.org/zap"
)

// setTarOwnership copies uid/gid onto header - tar.FileInfoHeader doesn't
// populate these from fs.FileInfo (not part of the portable interface), so
// without this every restored file lands owned by root regardless of
// original ownership, silently breaking any image (very common with
// LinuxServer.io-style images) that chowns and then permission-checks its
// data dir at startup.
func setTarOwnership(header *tar.Header, info os.FileInfo) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	header.Uid = int(stat.Uid)
	header.Gid = int(stat.Gid)
}

func checkFreeSpace(dir string, needed int64) error {
	if needed <= 0 {
		return nil
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		logger.Error("backup: failed to check free space, proceeding anyway", zap.Error(err))
		return nil
	}
	available := int64(stat.Bavail) * int64(stat.Bsize)
	if available < needed {
		return fmt.Errorf("not enough free space: need %s, have %s available", humanBytes(needed), humanBytes(available))
	}
	return nil
}
