package main

import (
	"archive/zip"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalln("usage: pack <dest.zip> <src dir>")
	}

	dstFile, _ := filepath.Abs(os.Args[1])

	file, err := os.Create(dstFile)
	if err != nil {
		log.Fatalln(err)
	}

	srcPath, _ := filepath.Abs(os.Args[2])
	srcPath += string(os.PathSeparator)

	writer := zip.NewWriter(file)

	err = filepath.Walk(srcPath, func(osFilePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		trunc := strings.ReplaceAll(strings.TrimPrefix(osFilePath, srcPath), "\\", "/")
		log.Println("Packing:", osFilePath, info.Mode())

		// FileInfoHeader keeps the Unix permission bits (e.g. the executable bit),
		// which writer.Create() would silently drop.
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = trunc
		header.Method = zip.Deflate

		fileWriter, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}

		fileReader, err := os.Open(osFilePath)
		if err != nil {
			return err
		}

		defer fileReader.Close()

		_, err = io.Copy(fileWriter, fileReader)

		return err
	})
	if err != nil {
		log.Fatalln("packing failed:", err)
	}

	if err = writer.Close(); err != nil {
		log.Fatalln(err)
	}

	if err = file.Close(); err != nil {
		log.Fatalln(err)
	}

	log.Println("Finished")
}
