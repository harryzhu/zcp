package cmd

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

func createZip(filelist []string, dbName string) (err error) {
	dbPath, _ := filepath.Abs(filepath.Join(LogDir, hashString([]byte(dbName))))
	if FileExists(dbPath) {
		err := os.Remove(dbPath)
		PrintError("createZip:os.Remove", err)
		return err
	}

	DebugInfo("createZip", dbPath)
	zipFileHandler, err := os.Create(dbPath)
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

			if err != nil {
				PrintError("createZip:io.Copy:"+fpath, err)
				continue
			}
			fp.Close()
		}

	}

	zw.Close()

	finfo, err = os.Stat(dbPath)
	if err != nil {
		PrintError("createZip:os.Stat", err)
		return err
	}

	PrintlnInfo("green", "createZip: Elapse", time.Since(t1), ", Size: ", finfo.Size()>>20, "MB")

	pbFile := file2pbFile(dbPath, finfo, "bolt")

	t1 = GetNowTime()
	err = pbBoltSend(dbPath, pbFile)
	if err != nil {
		PrintError("createZip:pbFileChunkSend", err)
		return err
	}
	PrintlnInfo("green", "createZip:send: Elapse", time.Since(t1))

	if FileExists(dbPath) {
		err := os.Remove(dbPath)
		PrintError("createZip:os.Remove", err)
		return err
	}

	return nil
}
