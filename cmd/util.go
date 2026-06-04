package cmd

import (
	"bufio"
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
	"github.com/zeebo/xxh3"
)

const (
	// for func IsPathValid
	PathBanSymbols string = `:\\*\">?<|`
)

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

func GetNowTime() time.Time {
	return time.Now()
}

func GetNowUnix() int64 {
	return time.Now().UTC().Unix()
}

func GetNowUnixMilli() int64 {
	return time.Now().UTC().UnixMilli()
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

func TimeStr2Unix(s string) int64 {
	layout := "2006-01-02 15:04:05"
	if strings.Contains(s, ",") {
		layout = "2006-01-02,15:04:05"
	}

	var parsedTime time.Time
	var err error

	parsedTime, err = time.ParseInLocation(layout, s, time.Local)

	if err != nil {
		PrintError("TimeStr2Unix", err)
		os.Exit(0)
	}

	return parsedTime.Unix()
}

func Int2Str(n int) string {
	return strconv.Itoa(n)
}

func Int32Str(n int32) string {
	return fmt.Sprintf("%v", n)
}

func Int64Int(n int64) int {
	n10, err := strconv.Atoi(strconv.FormatInt(n, 10))
	if err != nil {
		PrintError("Int64Int", err)
		return 0
	}
	return n10
}

func Int64Str(n int64) string {
	return fmt.Sprintf("%v", n)
}

func IsSymlink(src string) bool {
	linfo, err := os.Lstat(src)
	if err != nil {
		PrintError("getSymlink", err)
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
		PrintError("getSymlink", err)
		return ""
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		srcLinkTarget, err := os.Readlink(src)
		if err != nil {
			PrintError("getSymlink", err)
			return ""
		}
		return srcLinkTarget
	}
	return ""
}

func IsPathValid(p string) bool {
	if strings.ContainsAny(p, PathBanSymbols) {
		return false
	}
	return true
}

func IsFileNeeded(fpath string, finfo fs.FileInfo) bool {
	if FileExt != "" {
		if fextMatch.MatchString(filepath.Ext(fpath)) == false {
			return false
		}
	}

	if IsIgnoreDotFile == true {
		if strings.HasPrefix(filepath.Base(fpath), ".") {
			return false
		}
	}

	if finfo == nil {
		return true
	}

	if MinSize != -1 {
		if finfo.Size() < MinSize {
			return false
		}
	}

	if MaxSize != -1 {
		if finfo.Size() > MaxSize {
			return false
		}
	}

	if MinAge != "" {
		if finfo.ModTime().Unix() < TimeStr2Unix(MinAge) {
			return false
		}
	}

	if MaxAge != "" {
		if finfo.ModTime().Unix() > TimeStr2Unix(MaxAge) {
			return false
		}
	}

	return true
}

func GetFileSize(fpath string) int64 {
	finfo, err := os.Stat(fpath)
	if err != nil {
		PrintError("GetFileInfo", err)
		return 0
	}
	return finfo.Size()
}

func DumpFileList(flist []string, fname string) string {
	fpath := ToUnixSlash(filepath.Join(LogDir, fname))

	MakeDirs(filepath.Dir(fpath))
	dstWriter, err := os.OpenFile(fpath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		PrintError("dumpFileList: OpenFile", err)
		return ""
	}

	dstWriter.WriteString(strings.Join(flist, "\n"))

	dstWriter.Close()

	return fpath
}
