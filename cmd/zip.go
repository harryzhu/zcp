package cmd

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
)

func createZip(filelist []string, zname string) (err error) {
	zpath := ToUnixSlash(filepath.Join(LogDir, hashString([]byte(zname))))
	if Exists(zpath) {
		err := os.Remove(zpath)
		PrintError("createZip:os.Remove", err)
		return err
	}

	DebugInfo("createZip", zpath)
	zipFileHandler, err := os.Create(zpath)
	if err != nil {
		PrintError("createZip", err)
		return err
	}
	defer zipFileHandler.Close()

	compr := zstd.ZipCompressor(
		zstd.WithWindowSize(1<<20),
		zstd.WithEncoderConcurrency(4),
		zstd.WithEncoderLevel(zstd.SpeedDefault),
		zstd.WithEncoderCRC(false))

	zw := zip.NewWriter(zipFileHandler)
	zw.RegisterCompressor(zstd.ZipMethodWinZip, compr)
	zw.RegisterCompressor(zstd.ZipMethodPKWare, compr)

	t1 := time.Now()
	var fkey, fpath string
	var finfo os.FileInfo

	for _, relPath := range filelist {
		fkey = ToUnixSlash(strings.TrimPrefix(relPath, SourceDir))
		fpath = ToUnixSlash(filepath.Join(SourceDir, relPath))

		finfo, err = os.Stat(fpath)
		if err != nil {
			PrintError("createZip:os.Stat", err)
			continue
		}

		header, err := zip.FileInfoHeader(finfo)
		if err != nil {
			PrintError("createZip:zip.FileInfoHeader:"+fpath, err)
			continue
		}

		header.Name = fkey
		header.Method = zstd.ZipMethodWinZip

		w, err := zw.CreateHeader(header)
		PrintError("createZip:zw.CreateHeader", err)

		if !finfo.IsDir() {
			fp, err := os.Open(fpath)
			defer fp.Close()

			if err != nil {
				PrintError("createZip:os.Open:"+fpath, err)
				continue
			}
			_, err = io.Copy(w, fp)

			atomic.AddInt32(&totalNum, 1)
			atomic.AddInt64(&totalWriteSize, finfo.Size())

			if err != nil {
				PrintError("createZip:io.Copy:"+fpath, err)
				continue
			}
			fp.Close()
		}

	}

	zw.Close()

	finfo, err = os.Stat(zpath)
	if err != nil {
		PrintError("createZip:os.Stat", err)
		return err
	}

	PrintlnInfo("green", "createZip",
		"Elapse: ", time.Since(t1),
		", Files: ", len(filelist),
		", Zip: ", finfo.Size()>>20, "MB")

	pbFile := file2pbFile(zpath, "zip")

	t1 = GetNowTime()
	err = gClientStreamSend(zpath, pbFile)
	if err != nil {
		PrintError("createZip:gClientStreamSend", err)
		return err
	}
	PrintlnInfo("green", "createZip:send: Elapse", time.Since(t1))

	if Exists(zpath) {
		err := os.Remove(zpath)
		PrintError("createZip:os.Remove", err)
		return err
	}

	return nil
}
