package cmd

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
	"sync"
)

var (
	sendFailure sync.Map
)

func chunkSend(fpath string, action int32) error {
	_, err := os.Stat(fpath)
	if err != nil {
		PrintError("chunkSend: os.Stat", err)
		return err
	}
	fp, err := os.Open(fpath)
	if err != nil {
		PrintError("chunkSend: os.Open", err)
		return err
	}
	defer fp.Close()

	clientStream := GetClientStream()

	reader := bufio.NewReaderSize(fp, int(chunkSize))
	buffer := make([]byte, chunkSize)

	offset := int64(0)
	chunkNum := int32(0)
	pbf := file2pbFile(fpath, true)
	pbf.Action = 0
	for {
		n, err := reader.Read(buffer)
		if err == io.EOF || n == 0 {
			break
		}

		pbf.ChunkNum = chunkNum
		pbf.ChunkData = ZstdBytes(buffer[:n])
		pbf.ChunkOffset = offset
		pbf.ChunkSize = int64(n)

		if pbf.ChunkTotal == chunkNum+1 {
			pbf.Action = action
		}

		err = clientStream.Send(&pbf)
		FatalError("chunkSend: Send", err)

		offset += int64(n)
		chunkNum++
	}

	resp, err := clientStream.CloseAndRecv()
	if err != nil {
		FatalError("chunkSend: CloseAndRecv", err)
	}

	if resp.Action == -1 {
		sendFailure.Store(fpath, resp.Comment)
		PrintlnInfo("red", "chunkSend: ERROR", resp.Action, resp.Comment)
	}

	return nil
}

func chunkSave(pbFile *pb.File) error {
	targetPath := ToUnixSlash(filepath.Join(TargetDir, pbFile.Fpath))
	if !Exists(filepath.Dir(targetPath)) {
		MakeDirs(filepath.Dir(targetPath))
	}
	var targetPathTemp string = strings.Join([]string{targetPath, "ing"}, ".")

	var dstWriter *os.File
	var err error

	if pbFile.ChunkNum == 0 {
		if Exists(targetPathTemp) {
			err := os.Remove(targetPathTemp)
			if err != nil {
				PrintError("chunkSave", err)
				return err
			}
		}
		dstWriter, err = os.OpenFile(targetPathTemp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
		dstWriter.Truncate(pbFile.Fsize)
	} else {
		dstWriter, err = os.OpenFile(targetPathTemp, os.O_WRONLY, os.ModePerm)
	}

	if err != nil {
		dstWriter.Close()
		PrintError("chunkSave: os.OpenFile", err)
		return err
	}

	if pbFile.ChunkData != nil {
		cdata, err := UnZstdBytes(pbFile.ChunkData)
		if err != nil {
			PrintError("chunkSave: UnZstdBytes", err)
			return err
		}
		_, err = dstWriter.WriteAt(cdata, pbFile.ChunkOffset)
		if err != nil {
			dstWriter.Close()
			PrintError("chunkSave: WriteAt", err)
			return err
		}
		dstWriter.Close()
	}

	if pbFile.Action == 100 || pbFile.Action == 200 {
		dstWriter.Close()
		if hashFile(targetPathTemp) == pbFile.Fhash {
			err := os.Rename(targetPathTemp, targetPath)
			if err != nil {
				PrintError("chunkSave: os.Rename", err)
				return err
			}
		}

		if Exists(targetPath) {
			finfo := bytes2FileInfo(pbFile.Finfo)
			err = os.Chmod(targetPath, finfo.Mode)
			if err != nil {
				PrintError("chunkSave: os.Chmod", err)
				return err
			}

			err = os.Chtimes(targetPath, finfo.Mtime, finfo.Mtime)
			if err != nil {
				PrintError("chunkSave: os.Chtimes", err)
				return err
			}
		}
	}

	if pbFile.Action == 200 {
		extractZip(targetPath)
	}

	return nil
}
