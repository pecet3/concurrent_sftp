package multi_sftp

import (
	"errors"
	"log"
	"os"
	"sync"
	"time"
)

type Cache struct {
	maxSize int64
	cMu     sync.Mutex
	cMap    map[string]*File
	cSize   int64
}

func newCache(maxSize int64) *Cache {
	return &Cache{
		cMap:    make(map[string]*File),
		maxSize: maxSize,
	}
}
func (c *Cache) AddFile(key string, file *File) error {
	c.cMu.Lock()
	defer c.cMu.Unlock()
	c.cSize += file.Size
	if c.cSize > c.maxSize {
		return errors.New("max cache limit")
	}
	c.cMap[key] = file
	return nil
}

func (c *Cache) GetFile(key string) (*File, bool) {
	c.cMu.Lock()
	defer c.cMu.Unlock()
	file, exists := c.cMap[key]
	return file, exists
}

func (c *Cache) UpdateFileLastOpenAt(key string) bool {
	c.cMu.Lock()
	defer c.cMu.Unlock()

	file, exists := c.cMap[key]
	if !exists {
		return false
	}

	file.LastOpenAt = time.Now()
	return true
}

func (c *Cache) DeleteFile(key string) bool {
	c.cMu.Lock()
	defer c.cMu.Unlock()

	file, exists := c.cMap[key]
	if exists {
		delete(c.cMap, key)
	}
	c.cSize -= file.Size
	return exists
}

func (c *Cache) CleanUpLoop() {
	for {
		log.Println("<multi_sftp> DeletingCacheLoop is Running...")
		c.cMu.Lock()
		for _, f := range c.cMap {
			if time.Since(f.LastOpenAt) > time.Duration(time.Second*30) {
				err := os.Remove(f.Path + f.FileName)
				if err != nil {
					log.Println("<multi_sftp> DeletingCacheLoop Error", err)
					continue
				}
				c.cSize -= f.Size
				delete(c.cMap, f.FileName)
				log.Println("<multi_sftp> [DeletingCacheLoop] deleted file from cache: ", f.FileName)
			}
		}
		log.Printf("<multi_sftp> Total Cache Size: %dmb/10gb", c.cSize/(1024*1024))
		c.cMu.Unlock()

		time.Sleep(time.Minute * 60)
	}
}
