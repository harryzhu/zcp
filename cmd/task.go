package cmd

import (
	pb "pb"
)

func getChanFileToDisk(pbIn *pb.File) error {
	if isPathValid(string(pbIn.Path)) == false {
		err := NewError("path invalid: ", string(pbIn.Path))
		PrintError("getChanFileToDisk", err)

		safePbSaveStatus.Store(string(pbIn.Path), int64(412))
		return err
	}

	statusCode, err := pbFileChunkSave(pbIn)

	if err != nil {
		PrintError("getChanFileToDisk", err)
		safePbSaveStatus.Store(string(pbIn.Path), int64(500))
	} else {
		safePbSaveStatus.Store(string(pbIn.Path), int64(statusCode))
	}

	return nil
}
