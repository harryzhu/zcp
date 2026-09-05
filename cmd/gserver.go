package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	pb "pb"
)

type FileTransferService struct{}

func (s *FileTransferService) Head(ctx context.Context, pbIn *pb.File) (*pb.File, error) {
	resp := NewPbFile()

	if pbIn.Comment == "__HEALTHCHECK__" {
		resp.Action = 200
		return &resp, nil
	}

	dstPath := ToUnixSlash(filepath.Join(TargetDir, pbIn.Fpath))
	DebugInfo("Head", dstPath)

	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		resp.Action = -1
		return &resp, nil
	}

	if dstInfo.Size() != pbIn.Fsize {
		resp.Action = -1
		resp.Fsize = dstInfo.Size()
		return &resp, nil
	}

	h := hashFile(dstPath)
	if h != pbIn.Fhash {
		resp.Action = -1
		resp.Fhash = h
		return &resp, nil
	}

	resp.Action = 1

	return &resp, nil

}

func (s *FileTransferService) SyncMisc(ctx context.Context, miscIn *pb.Misc) (*pb.Misc, error) {
	resp := pb.Misc{}
	if miscIn.Type == "folder" {
		folderlist, err := Bytes2MapFinfoLite(miscIn.Data)
		if err == nil {
			for k, srcFinfo := range folderlist {
				targetFolder := ToUnixSlash(filepath.Join(TargetDir, k))
				DebugInfo("SyncMisc: folder", targetFolder)
				if !Exists(targetFolder) {
					MakeDirs(targetFolder)
				}

				err := os.Chmod(targetFolder, srcFinfo.Mode)
				PrintError("SyncMisc: folder", err)

				err = os.Chtimes(targetFolder, srcFinfo.Mtime, srcFinfo.Mtime)
				PrintError("SyncMisc: folder", err)
			}
		}

	}

	if miscIn.Type == "symlink" {
		symlist, err := Bytes2MapString(miscIn.Data)
		if err == nil {
			for k, dstLink := range symlist {
				srcPath := ToUnixSlash(filepath.Join(TargetDir, k))
				dstLink = ToUnixSlash(dstLink)
				MakeSymlink(dstLink, srcPath)
				DebugInfo("SyncMisc: MakeSymlink", dstLink, " <- ", srcPath)
			}
		}

	}

	return &resp, nil
}

func (s *FileTransferService) StreamReceive(stream pb.FileTransfer_StreamReceiveServer) error {
	for {
		pbIn, err := stream.Recv()
		if err == io.EOF {
			stream.SendAndClose(&pb.File{Action: 0, Comment: "OK"})
			return nil
		}

		if err != nil {
			stream.SendAndClose(&pb.File{Action: -1, Comment: err.Error()})
			return nil
		}

		err = chunkSave(pbIn)
		if err != nil {
			PrintError("StreamReceive", err)
		}

	}

}
