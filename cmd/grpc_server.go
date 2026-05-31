package cmd

import (
	"context"
	"io"
	"io/fs"
	"path/filepath"
	pb "pb"
	"strings"
	"time"
)

type FileInfoLite struct {
	Size    int64
	ModTime time.Time
	Mode    fs.FileMode
}

type FileTransferService struct{}

func (s *FileTransferService) Head(ctx context.Context, pbIn *pb.File) (*pb.File, error) {
	resp := NewPbFile()
	resp.Status = 0
	resp.Data = nil
	if string(pbIn.Ftype) == "FileHashList" {
		var m map[string]string
		pbInData, err := UnZstdBytes(pbIn.Data)
		if err != nil {
			PrintError("Head: UnZstdBytes", err)
			return &resp, err
		}
		filehashlist, err := Byte2MapStr(pbInData, m)
		if err != nil {
			PrintError("Head: Byte2MapStr", err)
			return &resp, err
		}
		var diffHashList map[string]string = make(map[string]string, 256)
		for spath, _ := range filehashlist {
			targetPath := filepath.Join(TargetDir, spath)
			if FileExists(targetPath) == false {
				diffHashList[spath] = "404"
				continue
			}
			if IsOverwrite == false {
				continue
			}
			diffHashList[spath] = hashFile(targetPath)
		}

		resp.Ftype = []byte("FileHashList")
		resp.Data = ZstdBytes(MapStr2Byte(diffHashList))
		PrintlnInfo("green", "FileHashList: Size", len(resp.Data))
		return &resp, nil
	}

	if string(pbIn.Ftype) == "file" {
		dstPath := pbGetTargetPath(pbIn)
		if FileExists(dstPath) {
			resp.Status = 200
			resp.Fsum = []byte(hashFile(dstPath))
		} else {
			resp.Status = 404
			resp.Fsum = nil
		}
	}

	return &resp, nil
}

func (s *FileTransferService) GetMisc(ctx context.Context, pbIn *pb.Misc) (*pb.Misc, error) {
	resp := &pb.Misc{}
	resp.Mtype = pbIn.Mtype
	resp.Data = nil

	reqType := pbIn.Mtype
	switch reqType {
	case "pbSaveStatus":
		safePbSaveStatus.Range(func(k, v any) bool {
			pbSaveStatus[k.(string)] = Int64Str(v.(int64))
			return true
		})
		resp.Data = MapStr2Byte(pbSaveStatus)
	case "ping":
		PrintlnInfo("HealthCheck from Client", string(pbIn.Data))
		if strings.Contains(string(pbIn.Data), "ping") {
			resp.Data = []byte("pong")
		} else {
			resp.Data = []byte("error")
		}

	default:
		resp.Data = []byte("ok")
	}

	return resp, nil
}

func (s *FileTransferService) PutMisc(ctx context.Context, pbIn *pb.Misc) (*pb.Misc, error) {
	resp := &pb.Misc{}
	resp.Mtype = pbIn.Mtype
	resp.Data = nil
	switch pbIn.Mtype {
	case "pbBolt":
		resp.Data = nil
	default:
		DebugInfo("PutMisc", "cannot match Mtype")
		resp.Data = nil
	}

	return resp, nil
}

func (s *FileTransferService) StreamReceive(stream pb.FileTransfer_StreamReceiveServer) error {
	emptyPbFile := NewPbFile()
	for {
		pbIn, err := stream.Recv()
		if err == io.EOF {
			//DebugInfo("StreamReceive", err)
			return nil
		}

		if err != nil || pbIn == nil {
			return nil
		}

		resp := emptyPbFile

		reqType := string(pbIn.Ftype)

		if reqType == "file" {
			// DebugInfo("StreamReceive: file", pbIn.ChunkNum, "/", pbIn.Chunks)
			getChanFileToDisk(pbIn)

			continue
		}

		if reqType == "dir" || reqType == "symlink" {
			resp.Status = pbFileDirSymlinkSave(pbIn)
			continue
		}

		if reqType == "bolt" {
			var boltPath string = ""
			boltPath, err = pbBoltSave(pbIn)
			PrintError("StreamReceive: pbBoltSave", err)
			if boltPath != "" {
				PrintlnInfo("green", "pbBoltExtract", boltPath)
				pbBoltExtract(boltPath)
			}

			continue
		}

		if reqType == "SIG" {
			PrintlnInfo("green", "received signal", string(pbIn.Comment))
			continue
		}

		DebugInfo("StreamReceive: resp.Status", resp.Status)

		err = stream.Send(&resp)
		if err != nil {
			PrintError("StreamReceive", err)
		}

	}
	return nil
}
