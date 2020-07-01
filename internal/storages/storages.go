/*
Package storages utility to load add stores for serinin.
*/
package storages

import (
	// load and register drivers of store for serinin.
	_ "github.com/koron/serinin/internal/binmemcachestore"
	_ "github.com/koron/serinin/internal/gocachestore"
	_ "github.com/koron/serinin/internal/memcachestore"
	_ "github.com/koron/serinin/internal/redisstore"
)
