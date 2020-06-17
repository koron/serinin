package storages

import (
	_ "github.com/koron/serinin/internal/binmemcachestore"
	_ "github.com/koron/serinin/internal/gocachestore"
	_ "github.com/koron/serinin/internal/memcachestore"
	_ "github.com/koron/serinin/internal/redisstore"
)
