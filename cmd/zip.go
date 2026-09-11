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

func createZip(filelist []string, taskId int32) (err error) {
	tid := atomic.LoadInt32(&taskId)
	zpath := strings.Join([]string{"_tmp_", Int32Str(tid), ".zst"}, "")
	if Exists(zpath) {
		err := os.Remove(zpath)
		PrintError("createZip:os.Remove", err)
		return err
	}

	zipFileHandler, err := os.Create(zpath)
	if err != nil {
		PrintError("createZip", err)
		return err
	}
	defer zipFileHandler.Close()

	compr := zstd.ZipCompressor(
		zstd.WithWindowSize(1<<20),
		zstd.WithEncoderConcurrency(8),
		zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithEncoderCRC(false))

	zw := zip.NewWriter(zipFileHandler)
	zw.RegisterCompressor(zstd.ZipMethodWinZip, compr)
	zw.RegisterCompressor(zstd.ZipMethodPKWare, compr)

	t1 := time.Now()
	var fkey, fpath string
	var finfo os.FileInfo
	SourceDir = ToUnixSlash(SourceDir)
	for _, fpath = range filelist {
		fpath = ToUnixSlash(fpath)
		fkey = strings.TrimPrefix(ToUnixSlash(strings.TrimPrefix(fpath, SourceDir)), "/")

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

		if finfo.IsDir() {
			continue
		}

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

	zw.Close()
	zipFileHandler.Close()

	finfo, err = os.Stat(zpath)
	if err != nil {
		PrintError("createZip:os.Stat", err)
		return err
	}

	PrintlnInfo("green", "createZip", zpath,
		" => Elapse: ", time.Since(t1),
		", Files: ", len(filelist),
		", Zip: ", finfo.Size()>>20, "MB")

	PrintlnInfo("green", "createZip", zpath, " => sending ...")
	t1 = GetNowTime()
	err = chunkSend(zpath, 200)
	if err != nil {
		PrintError("createZip:chunkSend", err)
		return err
	}
	tDuration := time.Since(t1).Seconds()
	speed := 0
	if tDuration > 0 {
		speed = int(float64(finfo.Size()) / tDuration)
	}
	PrintlnInfo("green", "createZip", zpath, " => Complete. ", time.Since(t1), ", ", speed>>20, "MB/s")

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
		PrintlnInfo("cyan", "extractZip:Size", finfo.Size(), " :", zipPath)
	}

	unzipReader, err := zip.NewReader(fh, finfo.Size())
	if err != nil {
		PrintError("extractZip:NewReader", err)
		return err
	}

	decomp := zstd.ZipDecompressor(
		zstd.WithDecoderConcurrency(8),
	)

	unzipReader.RegisterDecompressor(zstd.ZipMethodWinZip, decomp)
	unzipReader.RegisterDecompressor(zstd.ZipMethodPKWare, decomp)
	nSuccess := int32(0)
	nFailure := int32(0)
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
				atomic.AddInt32(&nFailure, 1)
				continue
			}
			funzip, err := fzip.Open()
			if err != nil {
				PrintError("extractZip:fzip.Open", err)
				atomic.AddInt32(&nFailure, 1)
				continue
			}

			if _, err := io.Copy(dst, funzip); err != nil {
				atomic.AddInt32(&nFailure, 1)
				PrintError("extractZip:io.Copy", err)
			}

			if err := funzip.Close(); err != nil {
				PrintError("extractZip:funzip.Close", err)
				atomic.AddInt32(&nFailure, 1)
			}
			dst.Close()
		}

		err = os.Chtimes(dstPath, finfo.ModTime(), finfo.ModTime())
		PrintError("extractZip:os.Chtimes", err)

		err = os.Chmod(dstPath, finfo.Mode())
		PrintError("extractZip:os.Chmod", err)
		atomic.AddInt32(&nSuccess, 1)
	}

	fh.Close()

	//
	if Exists(zipPath) {
		err := os.Remove(zipPath)
		PrintError("extractZip:os.Remove", err)
		return err
	}

	PrintlnInfo("green", "extractZip", filepath.Base(zipPath), "=> Success: ", atomic.LoadInt32(&nSuccess), ", Failure: ", atomic.LoadInt32(&nFailure))

	return nil
}
