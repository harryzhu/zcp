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

func createZip(filelist []string) (err error) {
	zpath := "_tmp.zst"
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

	for _, fpath = range filelist {
		fkey = strings.TrimPrefix(ToUnixSlash(strings.TrimPrefix(fpath, SourceDir)), "/")
		fpath = ToUnixSlash(fpath)

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
			if err != nil {
				PrintError("createZip:os.Open:"+fpath, err)
				continue
			}
			defer fp.Close()

			_, err = io.Copy(w, fp)

			if err != nil {
				PrintError("createZip:io.Copy:"+fpath, err)
				continue
			}
			fp.Close()
		}

	}

	zw.Close()
	zipFileHandler.Close()

	finfo, err = os.Stat(zpath)
	if err != nil {
		PrintError("createZip:os.Stat", err)
		return err
	}

	PrintlnInfo("green", "createZip",
		"Elapse: ", time.Since(t1),
		", Files: ", len(filelist),
		", Zip: ", finfo.Size()>>20, "MB")

	PrintlnInfo("green", "createZip", "sending zip ...")
	t1 = GetNowTime()
	err = chunkSend(zpath, 200)
	if err != nil {
		PrintError("createZip:chunkSend", err)
		return err
	}
	PrintlnInfo("green", "createZip: Send", time.Since(t1))

	if Exists(zpath) {
		err := os.Remove(zpath)
		PrintError("createZip:os.Remove", err)
		return err
	}

	return nil
}

func extractZip(zipPath string) error {
	fh, err := os.Open(zipPath)
	if err != nil {
		PrintError("extractZip:Open", err)
		return err
	}

	finfo, err := fh.Stat()
	if err != nil {
		PrintError("extractZip:Stat", err)
		return err
	} else {
		PrintlnInfo("cyan", "extractZip:Size", finfo.Size())
	}

	unzipReader, err := zip.NewReader(fh, finfo.Size())
	if err != nil {
		PrintError("extractZip:NewReader", err)
		return err
	}

	decomp := zstd.ZipDecompressor(
		zstd.WithDecoderConcurrency(4),
	)

	unzipReader.RegisterDecompressor(zstd.ZipMethodWinZip, decomp)
	unzipReader.RegisterDecompressor(zstd.ZipMethodPKWare, decomp)

	var dstPath string
	for _, fzip := range unzipReader.File {
		header := fzip.FileHeader
		finfo := header.FileInfo()

		dstPath = ToUnixSlash(filepath.Join(TargetDir, fzip.Name))
		if finfo.IsDir() {
			MakeDirs(dstPath)
		} else {
			MakeDirs(filepath.Dir(dstPath))
			dst, err := os.Create(dstPath)
			if err != nil {
				PrintError("extractZip:os.Create", err)
				continue
			}
			funzip, err := fzip.Open()
			if err != nil {
				PrintError("extractZip:fzip.Open", err)
				continue
			}

			if _, err := io.Copy(dst, funzip); err != nil {
				PrintError("extractZip:io.Copy", err)
			}

			if err := funzip.Close(); err != nil {
				PrintError("extractZip:funzip.Close", err)
			}
			dst.Close()
		}

		err = os.Chtimes(dstPath, finfo.ModTime(), finfo.ModTime())
		PrintError("extractZip:os.Chtimes", err)

		err = os.Chmod(dstPath, finfo.Mode())
		PrintError("extractZip:os.Chmod", err)
	}

	fh.Close()

	//
	if Exists(zipPath) {
		err := os.Remove(zipPath)
		PrintError("extractZip:os.Remove", err)
		return err
	}

	return nil
}
