package cmd

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/zeebo/xxh3"
)

type FinfoLite struct {
	Size  int64
	Mtime time.Time
	Mode  fs.FileMode
}

func NewFinfoLite(sz int64, mt time.Time, md fs.FileMode) FinfoLite {
	return FinfoLite{
		Size:  sz,
		Mtime: mt,
		Mode:  md,
	}
}

func fileInfo2Bytes(finfo fs.FileInfo) []byte {
	finfolite := FinfoLite{
		Size:  finfo.Size(),
		Mtime: finfo.ModTime(),
		Mode:  finfo.Mode(),
	}
	b, err := msgpack.Marshal(finfolite)
	FatalError("fileInfo2Bytes", err)
	return b
}

func bytes2FileInfo(b []byte) (flite FinfoLite) {
	err := msgpack.Unmarshal(b, &flite)
	FatalError("bytes2FileInfo", err)
	return flite
}

func ToUnixSlash(s string) string {
	// for windows
	return strings.ReplaceAll(s, "\\", "/")
}

func Exists(fpath string) bool {
	_, err := os.Stat(fpath)
	if err != nil {
		return false
	}
	return true
}

func GetFileSize(fpath string) int64 {
	finfo, err := os.Stat(fpath)
	if err != nil {
		return -1
	}
	return finfo.Size()
}

func ZstdBytes(rawin []byte) []byte {
	enc, _ := zstd.NewWriter(nil)
	return enc.EncodeAll(rawin, nil)
}

func UnZstdBytes(zin []byte) (out []byte, err error) {
	dec, _ := zstd.NewReader(nil)
	out, err = dec.DecodeAll(zin, nil)
	if err != nil {
		PrintError("UnZstdBytes:DecodeAll", err)
		return nil, err
	}
	return out, nil
}

func hashFile(fpath string) string {
	var hasher hash.Hash
	hasher = xxh3.New()

	fh, err := os.Open(fpath)
	if err != nil {
		PrintError("HashFile", err)
		return ""
	}

	r := bufio.NewReader(fh)

	var buf []byte = make([]byte, 8192)
	for {
		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			PrintError("HashFile", err)
		}
		hasher.Write(buf[:n])
	}

	fh.Close()
	return hex.EncodeToString(hasher.Sum(nil))
}

func hashString(b []byte) string {
	var hasher hash.Hash
	hasher = xxh3.New()
	hasher.Write(b)

	return hex.EncodeToString(hasher.Sum(nil))
}

func GetNowTimeStr(f string) string {
	switch f {
	case "Ymd":
		return time.Now().Format("20060102")
	case "H":
		return time.Now().Format("15")
	case "His":
		return time.Now().Format("150405")
	default:
		return time.Now().Format("20060102150405")
	}
}

func MakeDirs(dpath string) error {
	dpath = ToUnixSlash(dpath)
	_, err := os.Stat(dpath)
	if err != nil {
		DebugInfo("MakeDirs", dpath)
		err = os.MkdirAll(dpath, os.ModePerm)
		PrintError("MakeDirs:MkdirAll", err)
		return err
	}
	return nil
}

func GetNowTime() time.Time {
	return time.Now()
}

func Int2Str(n int) string {
	return strconv.Itoa(n)
}

func Int32Str(n int32) string {
	return fmt.Sprintf("%v", n)
}

func Int64Str(n int64) string {
	return fmt.Sprintf("%v", n)
}

func WriteFile(fp io.Writer, data []byte) error {
	r := bytes.NewReader(data)
	w := bufio.NewWriter(fp)
	_, err := w.ReadFrom(r)
	if err != nil {
		PrintError("BufferWriteFile", err)
		return err
	}
	w.Flush()
	return nil
}

func TimeStr2Unix(s string) int64 {
	layout := "2006-01-02 15:04:05"
	var parsedTime time.Time
	var err error

	parsedTime, err = time.ParseInLocation(layout, s, time.Local)

	if err != nil {
		FatalError("TimeStr2Unix", err)
	}

	return parsedTime.Unix()
}

func MakeSymlink(srcFile string, dstLink string) error {
	srcFile = ToUnixSlash(srcFile)
	dstLink = ToUnixSlash(dstLink)

	_, err := os.Lstat(dstLink)
	if err != nil {
		err := os.Symlink(srcFile, dstLink)
		if err != nil {
			PrintError("MakeSymlink", err)
			return err
		}
	}

	return nil
}

func IsSymlink(src string) bool {
	linfo, err := os.Lstat(src)
	if err != nil {
		PrintError("IsSymlink", err)
		return false
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return true
	}
	return false
}

func GetSymlink(src string) string {
	linfo, err := os.Lstat(src)
	if err != nil {
		PrintError("GetSymlink", err)
		return ""
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		srcLinkTarget, err := os.Readlink(src)
		if err != nil {
			PrintError("GetSymlink", err)
			return ""
		}
		return srcLinkTarget
	}
	return ""
}

func IsFileNeeded(fpath string, finfo fs.FileInfo) bool {
	if IsIgnoreDotFile == true {
		if strings.HasPrefix(filepath.Base(fpath), ".") {
			return false
		}
	}

	if FileExt != "" {
		if fextMatch.MatchString(filepath.Ext(fpath)) == false {
			return false
		}
	}

	fsize := finfo.Size()
	if MinSize != -1 {
		if fsize < MinSize {
			return false
		}
	}

	if MaxSize != -1 {
		if fsize > MaxSize {
			return false
		}
	}

	fmtime := finfo.ModTime().Unix()
	if MinAgeUnix != 0 {
		if fmtime < MinAgeUnix {
			return false
		}
	}

	if MaxAgeUnix != 0 {
		if fmtime > MaxAgeUnix {
			return false
		}
	}

	return true
}

func Map2Bytes(m map[string]any) ([]byte, error) {
	b, err := msgpack.Marshal(m)
	if err != nil {
		PrintError("Map2Bytes", err)
		return nil, err
	}
	return b, nil
}

func Bytes2MapString(b []byte) (map[string]string, error) {
	var m map[string]string
	err := msgpack.Unmarshal(b, &m)
	if err != nil {
		PrintError("Bytes2MapString", err)
		return nil, err
	}
	return m, nil
}

func Bytes2MapFinfoLite(b []byte) (map[string]FinfoLite, error) {
	var m map[string]FinfoLite
	err := msgpack.Unmarshal(b, &m)
	if err != nil {
		PrintError("Bytes2MapFinfoLite", err)
		return nil, err
	}
	return m, nil
}
