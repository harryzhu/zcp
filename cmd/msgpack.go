package cmd

import (
	"bytes"
	"io/fs"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

type FileInfoLite struct {
	Size    int64
	ModTime time.Time
	Mode    fs.FileMode
}

func finfo2FileInfoLite(finfo fs.FileInfo) []byte {
	filite := FileInfoLite{
		Size:    finfo.Size(),
		ModTime: finfo.ModTime(),
		Mode:    finfo.Mode(),
	}

	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	err := enc.Encode(filite)
	PrintError("file2pbFile: enc.Encode", err)

	return buf.Bytes()
}

func fileInfoLite2Finfo(fi []byte) (filite FileInfoLite, err error) {
	if fi != nil {
		buf := bytes.NewBuffer(fi)
		dec := msgpack.NewDecoder(buf)
		err := dec.Decode(&filite)
		if err != nil {
			PrintError("fileInfoLite2Finfo: dec.NewDecoder", err)
			return filite, err
		}
		return filite, nil
	}
	return filite, NewError("fi cannot be empty")
}

func Map2Byte(m map[string][]byte) []byte {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	err := enc.Encode(&m)
	if err != nil {
		PrintError("Map2Byte: enc.Encode", err)
		return nil
	}

	return buf.Bytes()
}

func Byte2Map(b []byte, m map[string][]byte) (map[string][]byte, error) {
	buf := bytes.NewBuffer(b)
	dec := msgpack.NewDecoder(buf)
	err := dec.Decode(&m)
	if err != nil {
		PrintError("Byte2Map: msgpack.NewDecoder", err)
		return m, err
	}
	return m, nil
}

func Byte2MapStr(b []byte) (map[string]string, error) {
	var m map[string]string
	err := msgpack.Unmarshal(b, &m)
	if err != nil {
		PrintError("Byte2MapStr", err)
		return m, err
	}
	return m, nil
}
