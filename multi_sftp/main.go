package multi_sftp

type Config struct {
	MaxCacheSize int64
	Workers      int
}
type MultiSFTP struct {
	SFTPmanager *SFTPmanager
	Cache       *Cache
}

func NewStorageManager(c Config) *MultiSFTP {
	sftpM := newSFTPmanager(c.Workers)
	cache := newCache(c.MaxCacheSize)
	return &MultiSFTP{
		SFTPmanager: sftpM,
		Cache:       cache,
	}
}
