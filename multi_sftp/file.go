package multi_sftp

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type File struct {
	Id         int       `json:"-"`
	Uuid       string    `json:"uuid"`
	IsNsfw     bool      `json:"is_nsfw"`
	IsPublic   bool      `json:"is_public"`
	FileName   string    `json:"file_name"`
	Path       string    `json:"-"`
	Url        string    `json:"url"`
	Size       int64     `json:"size"`
	Ext        string    `json:"ext"`
	IsTemp     bool      `json:"is_temp"`
	UserId     int       `json:"-"`
	ComesFrom  string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	LastOpenAt time.Time `json:"-"`
}

func (f File) RemoveFile(s *MultiSFTP, fDb *File) error {
	err := os.Remove(fDb.Path + fDb.FileName)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	s.Cache.DeleteFile(fDb.FileName)

	return nil
}

func (f File) DownloadAndSave(ctx context.Context,
	s *MultiSFTP, fDb *File, url string) (*File, error) {
	filePath := fDb.Path + fDb.FileName
	err := os.MkdirAll(fDb.Path, 0755)
	if err != nil {
		return nil, fmt.Errorf("could not create folder: %v", err)
	}
	log.Println(url, fDb.FileName)
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
	err = s.SFTPmanager.Upload(ctx, fDb, file)
	if err != nil {
		log.Println(err)
	}
	fDb.Size = size
	fmt.Printf("Image saved to %s\n, size:%d", filePath, fDb.Size)
	err = s.Cache.AddFile(fDb.FileName, fDb)
	if err != nil {
		log.Println(err)
	}
	return fDb, nil
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
	err = s.SFTPmanager.Upload(ctx, fDb, file)
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
func (f File) SaveFileForm(ctx context.Context, s *MultiSFTP, fh *multipart.FileHeader, fDb *File) error {
	file, err := fh.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	dst, err := os.Create(fDb.Path + fDb.FileName)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return err
	}
	err = s.SFTPmanager.Upload(ctx, fDb, dst)
	if err != nil {
		return err
	}

	return nil
}
