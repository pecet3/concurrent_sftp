package multi_sftp

type Config struct {
	MaxCacheSize    int64
	Workers         int
	IsDisabledCache bool
}
type MultiSFTP struct {
	Manager *Manager
	Cache   *Cache
}

func New(c Config) *MultiSFTP {
	sftpM := newManager(c.Workers)
	cache := newCache(c.MaxCacheSize)

	if c.IsDisabledCache {
		cache = nil
	}
	return &MultiSFTP{
		Manager: sftpM,
		Cache:   cache,
	}
}
