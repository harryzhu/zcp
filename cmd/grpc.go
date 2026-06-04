package cmd

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
)

func file2pbFile(fpath string, ftype string) *pb.File {
	pbFile := &pb.File{}
	finfo, err := os.Stat(fpath)
	if err != nil {
		PrintError("file2pbFile", err)
		return pbFile
	}
	//
	pbFile.Status = 0
	pbFile.Comment = nil
	pbFile.Path = []byte(strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/"))
	pbFile.Ftype = []byte(ftype)
	pbFile.Data = nil
	pbFile.Finfo = nil
	pbFile.Fsum = []byte(hashFile(fpath))

	//
	pbFile.OffsetFrom = 0
	pbFile.OffsetTo = 0
	if finfo != nil {
		pbFile.Fsize = finfo.Size()
		pbFile.OffsetTo = finfo.Size()
	}

	return pbFile
}

func pbGetTargetPath(pbIn *pb.File) (dstPath string) {
	TargetDir = strings.TrimSuffix(ToUnixSlash(TargetDir), "/")
	pbInPath := strings.TrimPrefix(ToUnixSlash(string(pbIn.GetPath())), "/")
	dstPath = strings.Join([]string{TargetDir, pbInPath}, "/")
	if string(pbIn.Ftype) == "zip" {
		dstPath = ToUnixSlash(filepath.Join(LogDir, hashString([]byte("zcp_server_zip"))))
	}

	return ToUnixSlash(dstPath)
}

func DiffSourceTarget(srcHashList map[string]string) map[string][]byte {
	diffHashList := make(map[string][]byte)
	var targetPath string
	for spath, _ := range srcHashList {
		targetPath = ToUnixSlash(filepath.Join(TargetDir, spath))
		if !Exists(targetPath) {
			diffHashList[spath] = []byte("404")
			continue
		}
		if IsOverwrite == false {
			continue
		}
		diffHashList[spath] = []byte(hashFile(targetPath))
	}
	return diffHashList
}

func pbFileChunkSave(pbIn *pb.File) (statusCode int, err error) {
	if pbIn.Path == nil {
		return 400, NewError("pbIn.Path cannot be empty")
	}

	if pbIn.Status < 0 {
		return int(pbIn.Status), nil
	}

	dstPath := ToUnixSlash(pbGetTargetPath(pbIn))
	dstPathTemp := strings.Join([]string{dstPath, "ing"}, ".")

	MakeDirs(filepath.Dir(dstPath))

	//DebugInfo("--- pbFileChunkSave", offFrom, " - ", offTo)
	if pbIn.Data == nil {
		PrintError("pbFileChunkSave: pbIn.Data", NewError("cannot be nil"))
		return 500, err
	}

	dstWriter, err := os.OpenFile(dstPathTemp, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if pbIn.OffsetFrom == 0 {
		dstWriter.Truncate(0)
		dstWriter.Truncate(pbIn.Fsize)
	}
	defer dstWriter.Close()

	if err != nil {
		PrintError("pbFileChunkSave: os.OpenFile", err)
		return 500, err
	}

	var pbInData []byte
	pbInData, err = UnZstdBytes(pbIn.Data)
	if err != nil {
		PrintError("pbFileChunkSave: UnZstdBytes", err)
		return 500, err
	}

	_, err = dstWriter.WriteAt(pbInData, pbIn.OffsetFrom)
	if err != nil {
		PrintError("pbFileChunkSave: dstWriter.Write", err)
		return 500, err
	}

	if pbIn.OffsetTo == pbIn.Fsize {
		dstWriter.Close()

		PrintlnInfo("green", "["+Int32Str(pbIn.Status)+"]", string(pbIn.Path))

		dstSum := hashFile(dstPathTemp)
		if string(pbIn.GetFsum()) != dstSum {
			err = NewError("dstPath xxhash is not matched",
				"server: ", dstSum, ", client: ", string(pbIn.Fsum))
			PrintError("streamSaveFile: dstSum", err)
			return 500, err
		}

		DebugInfo("streamSaveFile: dstSum matched", dstSum)
		if err = os.Rename(dstPathTemp, dstPath); err != nil {
			PrintError("streamSaveFile: os.Rename", err)
			return 500, err
		}

		fi := pbIn.GetFinfo()

		var pbInFinfo FileInfoLite
		pbInFinfo, err = fileInfoLite2Finfo(fi)
		if err != nil {
			PrintError("pbFileChunkSave: fileInfoLite2Finfo", err)
			return 500, err
		}

		if err = os.Chmod(dstPath, pbInFinfo.Mode); err != nil {
			PrintError("streamSaveFile: os.Chmod", err)
			return 500, err
		}

		if err = os.Chtimes(dstPath, pbInFinfo.ModTime, pbInFinfo.ModTime); err != nil {
			PrintError("streamSaveFile: os.Chtimes", err)
			return 500, err
		}

		DebugInfo("streamSaveFile: Saved", pbIn.OffsetTo, ": ", string(pbIn.Path))

		return 200, nil
	}

	return 206, nil
}

func pbFileZipExtract(zipPath string) error {
	fh, err := os.Open(zipPath)
	if err != nil {
		PrintError("pbFileZipExtract:Open", err)
		return err
	}

	finfo, err := fh.Stat()
	if err != nil {
		PrintError("pbFileZipExtract:Stat", err)
		return err
	} else {
		PrintlnInfo("cyan", "pbFileZipExtract:Size", finfo.Size())
	}

	unzipReader, err := zip.NewReader(fh, finfo.Size())
	if err != nil {
		PrintError("pbFileZipExtract:NewReader", err)
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
				PrintError("pbFileZipExtract:os.Create", err)
				continue
			}
			funzip, err := fzip.Open()
			if err != nil {
				PrintError("pbFileZipExtract:fzip.Open", err)
				continue
			}

			if _, err := io.Copy(dst, funzip); err != nil {
				PrintError("pbFileZipExtract:io.Copy", err)
			}

			if err := funzip.Close(); err != nil {
				PrintError("pbFileZipExtract:funzip.Close", err)
			}
			dst.Close()
		}

		err = os.Chtimes(dstPath, finfo.ModTime(), finfo.ModTime())
		PrintError("pbFileZipExtract:os.Chtimes", err)

		err = os.Chmod(dstPath, finfo.Mode())
		PrintError("pbFileZipExtract:os.Chmod", err)
	}

	fh.Close()

	//
	if Exists(zipPath) {
		err := os.Remove(zipPath)
		PrintError("pbFileZipExtract:os.Remove", err)
		return err
	}

	return nil
}

func ShowProgress() error {
	if IsDebug {
		return nil
	}
	pf := int32(0)
	for {
		if pf == 2 {
			break
		}
		time.Sleep(time.Second)
		pf = atomic.LoadInt32(&progressFlag)
		tnum := atomic.LoadInt32(&totalNum)
		tsize := atomic.LoadInt64(&totalWriteSize)
		tsizemb := int64(math.Round(float64(tsize) / 1024.0 / 1024.0))
		s := fmt.Sprintf("Total: %v, Size: %v MB",
			tnum,
			tsizemb)
		PrintSpinner2(":::", s)
	}

	return nil
}
