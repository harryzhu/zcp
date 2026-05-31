package cmd

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
)

func NewPbFile() pb.File {
	return pb.File{
		Status:   0,
		Comment:  nil,
		ChanNum:  0,
		Path:     nil,
		Ftype:    nil,
		Finfo:    nil,
		Fsum:     nil,
		Fsize:    0,
		Chunks:   0,
		ChunkNum: 0,
		Zstd:     false,
		Data:     nil,
	}
}

func pbGetTargetPath(pbIn *pb.File) (dstPath string) {
	TargetDir = strings.TrimSuffix(ToUnixSlash(TargetDir), "/")
	pbInPath := strings.TrimPrefix(ToUnixSlash(string(pbIn.GetPath())), "/")
	dstPath = strings.Join([]string{TargetDir, pbInPath}, "/")

	return ToUnixSlash(dstPath)
}

func pbBoltSend(fpath string, pbFile *pb.File) error {
	fp, err := os.Open(fpath)
	if err != nil {
		PrintError("pbBoltSend:os.Open", err)
		return err
	}
	defer fp.Close()

	reader := bufio.NewReaderSize(fp, chunkSize)
	buffer := make([]byte, chunkSize)

	chunkTotal := pbFile.Chunks
	chunkNum := 0

	for {
		n, err := reader.Read(buffer)

		if err != nil && err != io.EOF {
			PrintError("pbBoltSend:reader.Read", err)
			return err
		}

		if n == 0 || err == io.EOF {
			break
		}

		pbFile.Data = buffer[:n]
		pbFile.Zstd = false
		pbFile.ChunkNum = int32(chunkNum)

		err = gClientStream.Send(pbFile)
		if err != nil {
			PrintError("pbBoltSend: gClientStream.Send", err)
			return err
		}

		DebugInfo("pbBoltSend", chunkNum, "/", chunkTotal, " : ", n)
		chunkNum++

	}

	DebugInfo("pbBoltSend", "DONE. ", pbFile.ChunkNum, "/", chunkTotal)

	return nil
}

func pbBoltSave(pbIn *pb.File) (boltPath string, err error) {
	boltPath = filepath.Join(LogDir, hashString([]byte(strings.Join([]string{"rpcopy_server.db", "bolt"}, "_"))))

	chunkNum := pbIn.GetChunkNum()
	totalChunks := pbIn.GetChunks()

	var boltWriter *os.File
	if chunkNum == 0 {
		boltWriter, err = os.OpenFile(boltPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	} else {
		boltWriter, err = os.OpenFile(boltPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	}

	if err != nil {
		PrintError("pbBoltSave: ", err)
		return "", err
	}
	defer boltWriter.Close()

	DebugInfo("pbBoltSave", chunkNum, "/", totalChunks, " : ", len(pbIn.Data))

	_, err = boltWriter.Write(pbIn.Data)
	if err != nil {
		PrintError("pbBoltSave: ", err)
		return "", err
	}
	if chunkNum < totalChunks-1 {
		return "", nil
	}

	if chunkNum == totalChunks-1 {
		PrintlnInfo("blue", "pbBoltSave", chunkNum, "/", totalChunks, " : ", len(pbIn.Data))
		boltWriter.Close()
		return boltPath, nil
	}

	return "", NewError("bolt cannot save successfully")
}

func pbBoltExtract(boltPath string) (statusCode int, err error) {
	fh, err := os.Open(boltPath)
	if err != nil {
		PrintError("pbBoltExtract:Open", err)
		return 500, err
	}

	finfo, err := fh.Stat()
	if err != nil {
		PrintError("pbBoltExtract:Stat", err)
		return 500, err
	} else {
		PrintlnInfo("cyan", "pbBoltExtract:Size", finfo.Size())
	}

	unzipReader, err := zip.NewReader(fh, finfo.Size())
	if err != nil {
		PrintError("pbBoltExtract:NewReader", err)
		return 500, err
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
				PrintError("pbBoltExtract:os.Create", err)
				continue
			}
			funzip, err := fzip.Open()
			if err != nil {
				PrintError("pbBoltExtract:fzip.Open", err)
				continue
			}

			if _, err := io.Copy(dst, funzip); err != nil {
				PrintError("pbBoltExtract:io.Copy", err)
			}

			if err := funzip.Close(); err != nil {
				PrintError("pbBoltExtract:funzip.Close", err)
			}
			dst.Close()
		}

		err = os.Chtimes(dstPath, finfo.ModTime(), finfo.ModTime())
		PrintError("pbBoltExtract:os.Chtimes", err)

		err = os.Chmod(dstPath, finfo.Mode())
		PrintError("pbBoltExtract:os.Chmod", err)
	}

	fh.Close()

	//
	if FileExists(boltPath) {
		err := os.Remove(boltPath)
		PrintError("pbBoltExtract:os.Remove", err)
		return 0, err
	}

	return 0, nil
}

func finfo2FileInfoLite(finfo fs.FileInfo) []byte {
	filite := FileInfoLite{
		Size:    finfo.Size(),
		ModTime: finfo.ModTime(),
		Mode:    finfo.Mode(),
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(filite)
	PrintError("file2pbFile: enc.Encode", err)

	return buf.Bytes()
}

func fileInfoLite2Finfo(fi []byte) (filite FileInfoLite, err error) {
	if fi != nil {
		buf := bytes.NewBuffer(fi)
		dec := gob.NewDecoder(buf)
		err := dec.Decode(&filite)
		if err != nil {
			PrintError("fileInfoLite2Finfo: gob.NewDecoder", err)
			return filite, err
		}
		return filite, nil
	}
	return filite, NewError("fi cannot be empty")
}

func file2pbFile(fpath string, finfo fs.FileInfo, ftype string) *pb.File {
	pbFile := &pb.File{}
	//
	pbFile.Status = 0
	pbFile.Comment = nil
	if fpath == "" {
		pbFile.Path = nil
	} else {
		pbFile.Path = []byte(strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/"))
	}
	pbFile.Ftype = []byte(ftype)
	pbFile.ChunkNum = 0
	pbFile.Data = nil
	if ftype == "file" {
		pbFile.Fsum = []byte(hashFile(fpath))
	} else {
		pbFile.Fsum = nil
	}
	//
	if finfo == nil {
		pbFile.Chunks = 0
		pbFile.Fsize = 0
		pbFile.Finfo = nil
	} else {
		pbFile.Chunks = int32(math.Ceil(float64(finfo.Size()) / float64(chunkSize)))
		pbFile.Fsize = finfo.Size()
		pbFile.Finfo = finfo2FileInfoLite(finfo)
	}

	return pbFile
}

func serverHealthCheck() error {
	sp1 := GetNowTime()
	respMisc, err := gClient.GetMisc(context.Background(), &pb.Misc{Mtype: "ping", Data: []byte(strings.Join([]string{"ping from", Host}, " <= "))})

	if err != nil {
		PrintError("serverHealthCheck:", err)
		return err
	}
	var rt string
	if string(respMisc.Data) == "pong" {
		rt = "pong"
	}

	if string(respMisc.Data) == "error" {
		rt = "error"
	}
	PrintlnInfo("Cyan", "HealthCheck From Server", rt, ". [Latency]: ", time.Since(sp1))
	return nil
}

func pbFileChunkSend(fpath string, pbFile *pb.File) error {
	fp, err := os.Open(fpath)
	if err != nil {
		PrintError("pbFileChunkSend:os.Open", err)
		return err
	}
	defer fp.Close()

	reader := bufio.NewReaderSize(fp, chunkSize)
	buffer := make([]byte, chunkSize)

	atomic.AddInt32(&chanNum, 1)

	channum := atomic.LoadInt32(&chanNum)
	if channum > 3 {
		atomic.StoreInt32(&chanNum, 0)
		channum = 0
	}

	chunkTotal := pbFile.Chunks
	chunkNum := 0

	for {
		n, err := reader.Read(buffer)

		if err != nil && err != io.EOF {
			PrintError("pbFileChunkSend:reader.Read", err)
			return err
		}

		if n == 0 || err == io.EOF {
			break
		}

		pbFile.Data = ZstdBytes(buffer[:n])
		pbFile.Zstd = true

		pbFile.ChunkNum = int32(chunkNum)
		pbFile.ChanNum = channum

		err = gClientStream.Send(pbFile)
		if err != nil {
			PrintError("pbFileChunkSend: gClientStream.Send", err)
			return err
		}

		DebugInfo("pbFileChunkSend: chanNum", pbFile.ChanNum, " <- chunk: ", pbFile.ChunkNum, "/", chunkTotal, " : ", n)
		chunkNum++

	}

	DebugInfo("pbFileChunkSend", "ONE_DONE. ", pbFile.ChunkNum, "/", chunkTotal)

	return nil
}

func pbFileChunkSave(pbIn *pb.File) (statusCode int, err error) {
	DebugInfo("--- pbFileChunkSave: Received", pbIn.GetChanNum(), " <- ", pbIn.GetChunkNum(), "/", pbIn.GetChunks(), ": ", len(pbIn.Data))
	if TargetDir == "" {
		return 500, NewError("TargetDir cannot be empty")
	}

	if pbIn.Path == nil {
		return 400, NewError("pbIn.Path cannot be empty")
	}

	if pbIn.Status < 0 {
		return int(pbIn.Status), nil
	}

	dstPath := ToUnixSlash(pbGetTargetPath(pbIn))
	dstPathTemp := ToUnixSlash(strings.Join([]string{dstPath, "ing"}, "."))

	fi := pbIn.GetFinfo()

	var pbInFinfo FileInfoLite
	pbInFinfo, err = fileInfoLite2Finfo(fi)
	if err != nil {
		PrintError("pbFileChunkSave: fileInfoLite2Finfo", err)
		return 500, err
	}

	MakeDirs(filepath.Dir(dstPath))

	dstWriter, err := os.OpenFile(dstPathTemp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if pbIn.GetChunkNum() == 0 {
		dstWriter.Truncate(0)
	}
	defer dstWriter.Close()

	if err != nil {
		PrintError("pbFileChunkSave: os.OpenFile", err)
		return 500, err
	}

	chunkTotal := pbIn.GetChunks()
	chunkNum := pbIn.GetChunkNum()

	DebugInfo("--- pbFileChunkSave", chunkNum, "/", chunkTotal)
	if pbIn.Data == nil {
		PrintError("pbFileChunkSave", NewError("pbIn.Data cannot be empty"))
		return 500, NewError("pbIn.Data cannot be empty")
	}

	var pbInData []byte
	if pbIn.Zstd == true {
		pbInData, err = UnZstdBytes(pbIn.Data)
		if err != nil {
			PrintError("pbFileChunkSave: UnZstdBytes", err)
			return 500, err
		}
	} else {
		pbInData = pbIn.Data
	}

	_, err = dstWriter.Write(pbInData)
	if err != nil {
		PrintError("pbFileChunkSave: dstWriter.Write", err)
		return 500, err
	}

	if chunkNum == chunkTotal-1 {
		dstWriter.Close()

		dstSum := hashFile(dstPathTemp)
		if string(pbIn.GetFsum()) != dstSum {
			err = NewError("dstPath xxhash is not matched")
			PrintError("streamSaveFile: dstSum", err)
			return 500, err
		}

		DebugInfo("streamSaveFile: dstSum matched", dstSum)
		if err = os.Rename(dstPathTemp, dstPath); err != nil {
			PrintError("streamSaveFile: os.Rename", err)
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
		DebugInfo("streamSaveFile: Saved", chunkTotal, ": ", string(pbIn.Path))

		return 200, nil
	}

	return 206, nil
}

func pbFileDirSymlinkSave(pbIn *pb.File) (respStatus int32) {
	if string(pbIn.Ftype) == "dir" {
		dstPath := ToUnixSlash(filepath.Join(TargetDir, string(pbIn.GetPath())))
		//DebugInfo("dir", dstPath)
		MakeDirs(dstPath)

		if pbIn.Finfo != nil {
			finfo, err := fileInfoLite2Finfo(pbIn.Finfo)
			if err == nil {
				err = os.Chmod(dstPath, finfo.Mode)
				PrintError("pbFileDirSymlinkSave:dir:os.Chmod", err)
				err = os.Chtimes(dstPath, finfo.ModTime, finfo.ModTime)
				PrintError("pbFileDirSymlinkSave:dir:os.Chtimes", err)
			}
		}

		respStatus = 206
		return respStatus
	}

	if string(pbIn.Ftype) == "symlink" {
		pbInLink := string(pbIn.GetPath())
		pbInFile := string(pbIn.GetComment())
		pre := pbInFile[0:5]

		symLink := ToUnixSlash(filepath.Join(TargetDir, pbInLink))

		var srcFile string
		if pre == "RAW::" {
			srcFile = strings.TrimPrefix(pbInFile, "RAW::")
		}
		if pre == "SUB::" {
			srcFile = filepath.Join(TargetDir, strings.TrimPrefix(pbInFile, "SUB::"))
		}

		DebugInfo("pbFileDirSymlinkSave:symlink", pre, " => ", symLink, " => ", srcFile)
		MakeDirs(filepath.Dir(symLink))
		MakeSymlink(srcFile, symLink)

		respStatus = 206
		return respStatus
	}

	return 206
}
