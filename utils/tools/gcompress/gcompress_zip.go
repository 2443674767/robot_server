package gcompress

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 压缩文件
// files 文件数组，可以是不同dir下的文件或者文件夹
// dest 压缩文件存放地址
func ZipCompress(pack_path, dest string) error {
	d, _ := os.Create(dest)
	defer d.Close()
	zw := zip.NewWriter(d)
	defer zw.Close()
	//压缩成zip
	file, err := os.Open(pack_path)
	if err != nil {
		return err
	}
	defer file.Close()
	err = doCompress(file, "", zw)
	if err != nil {
		return err
	}
	return nil
}

func doCompress(file *os.File, prefix string, zw *zip.Writer) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		if prefix == "" {
			prefix = info.Name()
		} else {
			prefix = prefix + "/" + info.Name()
		}
		fileInfos, err := file.Readdir(-1)
		if err != nil {
			return err
		}
		for _, fi := range fileInfos {
			f, err := os.Open(file.Name() + "/" + fi.Name())
			defer f.Close()
			if err != nil {
				return err
			}
			err = doCompress(f, prefix, zw)
			if err != nil {
				return err
			}
		}
	} else {
		header, err := zip.FileInfoHeader(info)
		if prefix == "" {
			header.Name = header.Name
		} else {
			header.Name = prefix + "/" + header.Name
		}
		if err != nil {
			return err
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, file)
		file.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// UnZipContent 是解压zip文件，会使用压缩算法将“zippedContent”解压至“dstFolderPath”位置。
// 参数“dstFolderPath”应为一个目录。
// 可选参数“zippedPrefix”指定了“zippedFilePath”解压后的路径，
func UnZipFile(zippedFilePath, dstFolderPath string, zippedPrefix ...string) error {
	readerCloser, err := zip.OpenReader(zippedFilePath)
	if err != nil {
		return err
	}
	defer readerCloser.Close()
	return unZipFileWithReader(&readerCloser.Reader, dstFolderPath, zippedPrefix...)
}

func unZipFileWithReader(reader *zip.Reader, dstFolderPath string, zippedPrefix ...string) error {
	prefix := ""
	if len(zippedPrefix) > 0 {
		prefix = strings.ReplaceAll(zippedPrefix[0], `\`, `/`)
	}
	if err := os.MkdirAll(dstFolderPath, 0755); err != nil {
		return err
	}
	var (
		name    string
		dstPath string
		dstDir  string
	)
	for _, file := range reader.File {
		name = strings.ReplaceAll(file.Name, `\`, `/`)
		name = strings.Trim(name, "/")
		if prefix != "" {
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			name = name[len(prefix):]
		}
		dstPath = filepath.Join(dstFolderPath, name)
		if file.FileInfo().IsDir() {
			_ = os.MkdirAll(dstPath, file.Mode())
			continue
		}
		dstDir = filepath.Dir(dstPath)
		if len(dstDir) > 0 {
			if _, err := os.Stat(dstDir); os.IsNotExist(err) {
				if err = os.MkdirAll(dstDir, 0755); err != nil {
					return err
				}
			}
		}
		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		// The fileReader is closed in function doCopyForUnZipFileWithReader.
		if err = doCopyForUnZipFileWithReader(file, fileReader, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func doCopyForUnZipFileWithReader(file *zip.File, fileReader io.ReadCloser, dstPath string) error {
	defer fileReader.Close()
	targetFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err = io.Copy(targetFile, fileReader); err != nil {
		return err
	}
	return nil
}
