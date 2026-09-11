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

	if pbIn.Comment == "__HEALTHCHECK__" {
		resp.Action = 200
		resp.Comment = strings.Join([]string{"server", runPlatform}, ",")
		return &resp, nil
	}

	dstPath := ToUnixSlash(filepath.Join(TargetDir, pbIn.Fpath))
	//DebugInfo("Head", dstPath)

	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		resp.Action = -1
		resp.Comment = "404"
		return &resp, nil
	}

	if dstInfo.IsDir() {
		resp.Action = 0
		resp.Comment = "path cannot be dir"
		return &resp, nil
	}

	if dstInfo.Size() != pbIn.Fsize {
		resp.Action = -1
		resp.Fsize = dstInfo.Size()
		resp.Comment = "different size"
		return &resp, nil
	}

	h := hashFile(dstPath)
	if pbIn.Fhash == "" {
		resp.Action = 0
		resp.Fhash = h
		resp.Comment = "TBD by hash"
		return &resp, nil
	}

	if h == pbIn.Fhash {
		resp.Action = 1
		resp.Fhash = h
		resp.Comment = "same"
		return &resp, nil
	}

	resp.Action = -1
	resp.Comment = "different hash"
	DebugInfo("HEAD", "[different]", dstPath)

	return &resp, nil

}

func (s *FileTransferService) SyncMisc(ctx context.Context, miscIn *pb.Misc) (*pb.Misc, error) {
	resp := pb.Misc{}
	if miscIn.Type == "folder" {
		folderlist, err := Bytes2MapFinfoLite(miscIn.Data)
		PrintError("SyncMisc: folder", err)
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
		PrintError("SyncMisc: symlink", err)
		if err == nil {
			for sym, dstFile := range symlist {
				sym = ToUnixSlash(filepath.Join(TargetDir, sym))
				MakeSymlink(dstFile, sym)
				DebugInfo("SyncMisc: MakeSymlink", "[file]: ", dstFile, " <=== [symbolink]: ", sym)
			}
		}
	}

	if miscIn.Type == "difflist" {
		resp.Type = "difflist"
		resp.Data = nil
		pathsize, err := Bytes2MapString(miscIn.Data)
		PrintError("SyncMisc:difflist", err)
		if err == nil {
			var dstPath string
			var dstSize int64
			var dstInfo os.FileInfo
			var result map[string]any = make(map[string]any, len(pathsize))
			for relpath, srcsize := range pathsize {
				dstPath = ToUnixSlash(filepath.Join(TargetDir, relpath))
				dstInfo, err = os.Stat(dstPath)
				if err != nil {
					result[relpath] = "-1"
					DebugInfo("SyncMisc:Diff:404", relpath)
					continue
				}
				if dstInfo.IsDir() {
					result[relpath] = "0"
					DebugInfo("SyncMisc:Diff:Folder", relpath)
					continue
				}
				dstSize = GetFileSize(dstPath)
				if Int64Str(dstSize) != srcsize {
					if IsSymlink(dstPath) == false {
						result[relpath] = "-1"
						DebugInfo("SyncMisc:Diff:Size", "dstSize:", dstSize, ",srcsize: ", srcsize, " <= ", relpath)
						continue
					}
				}
				result[relpath] = hashFile(dstPath)
			}

			b, err := Map2Bytes(result)
			if err != nil {
				PrintError("SyncMisc:Diff", err)
			} else {
				resp.Data = b
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
			PrintError("StreamReceive", err)
			stream.SendAndClose(&pb.File{Action: -1, Comment: err.Error()})
			return err
		}

		err = chunkSave(pbIn)
		if err != nil {
			PrintError("StreamReceive", err)
			stream.SendAndClose(&pb.File{Action: -1, Comment: err.Error()})
		}

	}
}
