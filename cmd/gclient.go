package cmd

import (
	"io/fs"
	"path/filepath"
	"strings"
)

func ClientWalkSourceDir() error {
	smallCount := 0
	largeCount := 0
	skipCount := 0
	sumSize := int64(0)
	filepath.WalkDir(SourceDir, func(fpath string, dirInfo fs.DirEntry, err error) error {
		if err != nil {
			PrintError("ClientWalkSourceDir", err)
			return err
		}
		fpath = ToUnixSlash(fpath)

		if IsFollowSymlink == false {
			if IsSymlink(fpath) {
				linkTo := GetSymlink(fpath)
				if linkTo != "" {
					symList[fpath] = strings.Join([]string{"RAW", linkTo}, "::")
					if strings.HasPrefix(linkTo, SourceDir) {
						t1 := strings.TrimPrefix(linkTo, SourceDir)
						symList[fpath] = strings.Join([]string{"SUB", t1}, "::")
					}
				}
				return nil
			}
		}

		finfo, err := dirInfo.Info()
		if err != nil {
			PrintError("ClientWalkSourceDir: dirInfo.Info", err)
		}
		if dirInfo.IsDir() {
			dirList[fpath] = finfo
			return nil
		}

		if IsFileNeeded(fpath, finfo) == false {
			skipCount++
			return nil
		}

		relPath := strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")
		fsize := finfo.Size()
		if fsize > maxSmallFileSize {
			chanLargeFile <- relPath
			largeCount++
		} else {
			chanSmallFile <- relPath
			smallCount++
		}

		sumSize += fsize

		return nil
	})

	chanLargeFile <- "_ALLDONE_"
	chanSmallFile <- "_ALLDONE_"

	PrintlnInfo("purple", "Processing", "smallFiles: ", smallCount,
		", largeFiles: ", largeCount,
		", skipFiles: ", skipCount,
		", sumSize: ", sumSize>>20, "MB")

	return nil
}

func ClientSendLargeFiles() error {
	taskSendLargeFiles()
	return nil
}

func ClientSendSmallFiles() error {
	taskSendSmallFiles()
	return nil
}

func ClientSendDirSymlink() error {
	var mapDir, mapSym map[string][]byte
	mapDir = make(map[string][]byte, len(dirList))
	mapSym = make(map[string][]byte, len(symList))
	DebugInfo("ClientSendDirSymlink: Sending", "dir list")
	for k, v := range dirList {
		k = ToUnixSlash(strings.TrimPrefix(k, SourceDir))
		mapDir[k] = finfo2FileInfoLite(v)
	}
	mdir := NewPbMisc("dir", Map2Byte(mapDir))
	gClientGetMisc(&mdir)

	//
	DebugInfo("ClientSendDirSymlink: Sending", "sym list")
	for slink, sfile := range symList {
		slink = strings.TrimLeft(ToUnixSlash(strings.TrimPrefix(slink, SourceDir)), "/")
		mapSym[slink] = []byte(sfile)
	}
	msym := NewPbMisc("sym", Map2Byte(mapSym))
	gClientGetMisc(&msym)

	return nil
}
