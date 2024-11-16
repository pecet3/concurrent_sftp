package multi_sftp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type File struct {
	FileName   string
	Path       string
	Size       int64
	Ext        string
	CreatedAt  time.Time
	LastOpenAt time.Time
}

func (f *File) DownloadAndSave(ctx context.Context,
	s *MultiSFTP, url string) (*File, error) {
	filePath := f.Path + f.FileName
	err := os.MkdirAll(f.Path, 0755)
	if err != nil {
		return nil, fmt.Errorf("could not create folder: %v", err)
	}
	log.Println(url, f.FileName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("could not fetch image: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not create file: %v", err)
	}
	size, err := io.Copy(file, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not save image: %v", err)
	}
	defer file.Close()
	err = s.Manager.Upload(ctx, f, file)
	if err != nil {
		log.Println(err)
	}
	f.Size = size
	fmt.Printf("Image saved to %s\n, size:%d", filePath, f.Size)
	err = s.Cache.AddFile(f.FileName, f)
	if err != nil {
		log.Println(err)
	}
	return f, nil
}

func (s File) saveBytes(data io.Reader, folderPath, fileName string) (int64, string, error) {
	err := os.MkdirAll(folderPath, 0755)
	if err != nil {
		return 0, "", nil
	}
	filePath := fmt.Sprintf("%s%v", folderPath, fileName)
	dst, err := os.Create(filePath)
	if err != nil {
		return 0, "", err
	}
	defer dst.Close()
	size, err := io.Copy(dst, data)
	if err != nil {
		return 0, "", nil
	}

	return size, filePath, nil
}
func (f File) SaveBytes(s *MultiSFTP,
	data io.Reader, fDb *File) (*File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	size, filePath, err := f.saveBytes(data, fDb.Path, fDb.FileName)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	fDb.Size = size
	log.Println(size)
	file, err := os.Open(filePath)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	err = s.Manager.Upload(ctx, fDb, file)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	err = s.Cache.AddFile(fDb.FileName, fDb)
	if err != nil {
		return nil, err
	}
	return fDb, nil
}
