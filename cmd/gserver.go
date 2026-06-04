package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
)

type FileTransferService struct{}

func (s *FileTransferService) Head(ctx context.Context, pbIn *pb.File) (*pb.File, error) {
	resp := NewPbFile()
	resp.Status = 0
	resp.Data = nil

	pbInFtype := string(pbIn.Ftype)
	if pbInFtype == "file" {
		dstPath := pbGetTargetPath(pbIn)
		if Exists(dstPath) {
			resp.Status = 200
			resp.Fsum = []byte(hashFile(dstPath))
		} else {
			resp.Status = 404
			resp.Fsum = []byte("")
		}
		return &resp, nil
	}

	return &resp, nil
}

func (s *FileTransferService) GetMisc(ctx context.Context, pbIn *pb.Misc) (*pb.Misc, error) {
	resp := &pb.Misc{}
	resp.Mtype = "error"
	resp.Data = nil

	var pbInData []byte
	var err error
	if pbIn.Data != nil {
		PrintlnInfo("green", "Received misc", len(pbIn.Data))
		pbInData, err = UnZstdBytes(pbIn.Data)
		if err != nil {
			PrintError("GetMisc: UnZstdBytes", err)
			return resp, err
		}
	}

	var respData []byte

	switch pbIn.Mtype {
	case "ping":
		PrintlnInfo("green", "HealthCheck from Client", string(pbInData))
		respData = []byte("OK")
	case "FileHashList":
		inHash, err := Byte2MapStr(pbInData)
		if err != nil {
			PrintError("GetMisc: FileHashList: Byte2MapStr", err)
		}
		diffHashList := DiffSourceTarget(inHash)
		respData = Map2Byte(diffHashList)
	case "dir":
		var mdir map[string][]byte
		inDir, err := Byte2Map(pbInData, mdir)
		if err != nil {
			PrintError("GetMisc: dir: Byte2MapStr", err)
		}
		for spath, v := range inDir {
			finfo, err := fileInfoLite2Finfo(v)
			if err != nil {
				PrintError("GetMisc: dir: fileInfoLite2Finfo", err)
				continue
			}
			targetPath := ToUnixSlash(filepath.Join(TargetDir, spath))
			MakeDirs(targetPath)
			DebugInfo("GetMisc: dir", targetPath)
			err = os.Chtimes(targetPath, finfo.ModTime, finfo.ModTime)
			PrintError("GetMisc: dir: os.Chtimes", err)
			err = os.Chmod(targetPath, finfo.Mode)
			PrintError("GetMisc: dir: os.Chtimes", err)
		}
	case "sym":
		var msym map[string][]byte
		inSym, err := Byte2Map(pbInData, msym)
		if err != nil {
			PrintError("GetMisc: sym: Byte2MapStr", err)
		}
		for slink, sfile := range inSym {
			strsfile := string(sfile)
			DebugInfo("GetMisc: sym", slink, ": ", strsfile)
			if len(strsfile) > 5 {
				pre := strsfile[0:5]
				targetLink := ToUnixSlash(filepath.Join(TargetDir, slink))
				//
				var srcFile string
				if pre == "RAW::" {
					srcFile = strings.TrimPrefix(strsfile, "RAW::")
				}
				if pre == "SUB::" {
					srcFile = filepath.Join(TargetDir, strings.TrimPrefix(strsfile, "SUB::"))
				}
				DebugInfo("GetMisc: sym", pre, " => ", targetLink, " => ", srcFile)
				MakeDirs(filepath.Dir(targetLink))
				MakeSymlink(srcFile, targetLink)
			}
		}

	default:
		respData = []byte("UNKNOWN")
	}

	resp.Mtype = pbIn.Mtype
	resp.Data = ZstdBytes(respData)

	return resp, nil
}

func (s *FileTransferService) PutMisc(ctx context.Context, pbIn *pb.Misc) (*pb.Misc, error) {
	resp := &pb.Misc{}
	resp.Mtype = "error"
	resp.Data = nil

	return resp, nil
}

func (s *FileTransferService) StreamReceive(stream pb.FileTransfer_StreamReceiveServer) error {
	emptyPbFile := NewPbFile()
	for {
		pbIn, err := stream.Recv()
		if err == io.EOF {
			respDone := emptyPbFile
			respDone.Status = 200
			stream.SendAndClose(&respDone)
			return nil
		}

		if err != nil {
			return nil
		}

		pbInFtype := string(pbIn.Ftype)

		if pbInFtype == "file" {
			pbFileChunkSave(pbIn)
			continue
		}

		if pbInFtype == "zip" {
			pbFileChunkSave(pbIn)
			zipPath := pbGetTargetPath(pbIn)
			if Exists(zipPath) {
				pbFileZipExtract(zipPath)
			}
			continue
		}

	}
	return nil
}
