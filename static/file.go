package static

import (
	"dev.risinghf.com/go/framework/log"
	"embed"
	"io"
	"os"
	"strings"
)

//go:embed *.dll
var staticFiles embed.FS

func init() {
	siteFile, err := staticFiles.Open("wintun.dll")
	if err != nil {
		log.Error(err)
		return
	}

	defer siteFile.Close()
	// 读取文件内容
	siteData, err := io.ReadAll(siteFile)
	if err != nil {
		log.Error(err)
		return
	}
	sitePath := CurrentPath() + "/wintun.dll"
	log.Info("start sync file", sitePath)
	f, err := os.Stat(sitePath)
	if os.IsNotExist(err) {
		err = write(sitePath, siteData)
		if err != nil {
			log.Error(err)
		}
		return
	}
	if f.Size() != int64(len(siteData)) {
		err = write(sitePath, siteData)
		if err != nil {
			log.Error(err)
		}
	}
}

func write(filename string, content []byte) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	_, err = file.Write(content)
	if err != nil {
		return err
	}
	err = file.Sync()
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func CurrentPath() string {
	return getCurrentPath()
}

func getCurrentPath() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}
