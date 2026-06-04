package cmd

import (
	"bufio"
	"context"
	"io"
	"os"
	pb "pb"
	"time"
)

func NewPbFile() pb.File {
	return pb.File{
		Status:     0,
		Comment:    nil,
		Path:       nil,
		Ftype:      nil,
		Finfo:      nil,
		Fsum:       nil,
		Fsize:      0,
		OffsetFrom: 0,
		OffsetTo:   0,
		Data:       nil,
	}
}

func NewPbMisc(mtype string, data []byte) pb.Misc {
	m := pb.Misc{}
	if mtype != "" {
		m.Mtype = mtype
	}
	if data != nil {
		m.Data = ZstdBytes(data)
	}
	return m
}

func gClientHead(pbFile *pb.File) (statusCode int32, fsum string) {
	statusCode = 500
	fsum = ""
	resp, err := GetClient().Head(context.Background(), pbFile)

	if err != nil {
		PrintError("gClientHead:", err)
		return 500, ""
	}

	statusCode = resp.Status
	fsum = string(resp.Fsum)

	return statusCode, fsum
}

func gClientGetMisc(miscIn *pb.Misc) []byte {
	resp, err := GetClient().GetMisc(context.Background(), miscIn)

	if err != nil {
		PrintError("gClientGetMisc:", err)
		return nil
	}

	respData, err := UnZstdBytes(resp.Data)
	if err != nil {
		PrintError("gClientGetMisc:UnZstdBytes", err)
		return nil
	}

	return respData
}

func gClientStreamSend(fpath string, pbFile *pb.File) error {
	finfo, err := os.Stat(fpath)
	if err != nil {
		PrintError("gClientStreamSend:os.Stat", err)
		return err
	}

	fp, err := os.Open(fpath)
	if err != nil {
		PrintError("gClientStreamSend:os.Open", err)
		return err
	}
	defer fp.Close()

	clientStream := GetClientStream()

	reader := bufio.NewReaderSize(fp, chunkSize)
	buffer := make([]byte, chunkSize)

	offFrom := int64(0)
	offTo := int64(0)
	for {
		n, err := reader.Read(buffer)

		if err != nil && err != io.EOF {
			return err
		}

		if n == 0 || err == io.EOF {
			break
		}

		offTo = offFrom + int64(n)

		if offTo == finfo.Size() {
			pbFile.Status = 100
			pbFile.Finfo = finfo2FileInfoLite(finfo)
		} else {
			pbFile.Status = 10
			pbFile.Finfo = nil
		}

		pbFile.OffsetFrom = offFrom
		pbFile.OffsetTo = offTo
		pbFile.Data = ZstdBytes(buffer[:n])

		err = clientStream.Send(pbFile)
		if err != nil {
			PrintError("gClientStreamSend: gClientStream.Send", err)
			return err
		}

		offFrom += int64(n)
	}

	resp, err := clientStream.CloseAndRecv()
	if err != nil {
		PrintError("gClientStreamSend: gClientStream.CloseAndRecv", err)
		return err
	}

	DebugInfo("gClientStreamSend", "ONE_DONE. ",
		"["+Int32Str(resp.Status)+"]", " <= ", string(pbFile.Path))

	return nil
}

func serverHealthCheck() error {
	if IsWithTLS {
		gClientConn = _buildGrpcTLSClientConn()
	} else {
		gClientConn = _buildGrpcClientConn()
	}

	sp1 := GetNowTime()
	clientName, err := os.Hostname()

	PrintError("serverHealthCheck: os.Hostname", err)

	miscIn := NewPbMisc("ping", []byte(clientName))

	status := "ERROR"
	resp := gClientGetMisc(&miscIn)
	if resp != nil {
		status = string(resp)
	}

	PrintlnInfo("Cyan", "HealthCheck From Server", status, ". [Latency]: ", time.Since(sp1))
	return nil
}
