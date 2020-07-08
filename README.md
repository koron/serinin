# serinin - multicast HTTP request

[![Actions/Go](https://github.com/koron/serinin/workflows/Go/badge.svg)](https://github.com/koron/serinin/actions?query=workflow%3AGo)
[![Go Report Card](https://goreportcard.com/badge/github.com/koron/serinin)](https://goreportcard.com/report/github.com/koron/serinin)

## Getting started

### With pre-compiled binary

1. Download an archive from <https://github.com/koron/serinin/releases/latest>
2. Extract `serinin` from the archive and isntall to one of directories in your `PATH`
3. Copy `serinin_config-sample.json` from the archive as `serinin_config.json`

    ```console
    cp serinin_config-sample.json serinin_config.json
    ```

4. Edit `serinin_config.json` which copied at step 3. See below section for [configuration](#Configuration)
5. Start serinin

    ```console
    $ serinin -config /usr/local/etc/serinin_config.json
    ```

    `-config` option gives a path of configuration file, the default is
    `serinin_config.json` in current directory.

### From source code

1. Download, build, and install with:

    ```
    $ go get -u github.com/koron/serinin
    ```

2. Copy `serinin_config-sample.json` as `serinin_config.json`

    ```
    $ cp $GOPATH/src/github.com/koron/serinin/serinin_config-sample.json serinin_config.json
    ```

3. Edit `serinin_config.json` which copied at step 2. See below section for [configuration](#Configuration)
4. Start serinin

    ```console
    $ serinin
    ```

    It starts with `serinin_config.json` in current directory.

## Options

```
  -config string
        path of configuration file (default "serinin_config.json")
  -handler int
        override max_handlers configuration if larger than zero
  -monitor int
        enable monitoring (poll system metric in each N's second)
  -storetype string
        override store_type configuration if not empty
  -worker int
        override worker_num configuration if larger than zero
```

### How to tuning

1. determine `-handler` depending your CPU and network speed
2. determine `-worker`. It would be better that multiply number of -handler by number of endpoints
3. for `-storetype`, `binmemcached` is best for now

## Configuration

See [`config.schema.json`](./config.schema.json) for the schema of configuration.
It is wrote in JSON Schema format.

### Commentary Configuration

```javascript
{
  // listen port of serinin HTTP server (mandatory)
  "addr": ":8000",

  // timeout for graceful shutdown (mandatory)
  "shutdown_timeout": "30s",

  // number of active handlers on serinin HTTP server. (optional)
  // overridable by `-handler` option.
  "max_handlers": 16,

  // number of workers to access endpoints. (optional)
  // overridable by `-worker` option.
  "worker_num": 96,

  // default timeout for accessing endpoints (optional)
  "http_client_timeout": "500ms",

  // information of endpoints. key is endpoint's name,
  // value is information of an endpoint.
  "endpoints": {

    "ep1": {
      // URL of an endpoint (mandatory)
      "url": "http://endpoint1.example.org:80/",

      // Timeout for an endpoint. (optional)
      // default is same with "http_client_timeout".
      "timeout": "1s",
    },

    "ep2": {
      // ...(snip)...
    },

    // add endpoints at here as you need

    // store type to store responses from endpoints. (mandatory)
    // possible values are:
    //
    //  * "binmemcache" - use memcached as store with binary protocol.
    //  * "redis" - use redis as store.
    //  * "memcache" - use memcached as store with text protocol.
    //      bit unstable under high load.
    //  * "gocache" - in memory cache, just for benchmark.
    //  * "none" - not store, just for benchmark.
    "store_type": "binmemcache",

    // one of store type configuration is mandatory,
    // depending on choice of "store_type".

    // configuration for "binmemcache" store type.
    "binmemcache": {
      // addresses of memcached nodes. (mandatory)
      "addrs": [ "127.0.0.1:11211" ],

      // life time for endpoint's responses (mandatory)
      "expire_in": "60s",

      // number of connection per memcached nodes. (optional)
      "conns_per_node": 100,
    },

    // configuration for "redis" store type.
    "redis": {
      // address of redis-server (mandatory)
      "addr": "127.0.0.1:6739",

      // password for connecting redis-server (optional)
      "password": "abcd1234",

      // redis's database number, default is zero (optional)
      "dbnum": 0,

      // life time for endpoint's responses (mandatory)
      "expire_in": "60s",

      // size of connection pool. (optional)
      // it would work better that `handler * (endpoints + 1)`
      "pool_size": 100,
    },

    // configuration for "memcache" store type.
    "memcache": {
      // addresses of memcached nodes. (mandatory)
      "addrs": [ "127.0.0.1:11211" ],

      // life time for endpoint's responses (mandatory)
      "expire_in": "60s",

      // max number of idle connections (optional)
      "max_idle_conns": 200,
    },

    // configuration for "gocache" store type.
    "gocache": {
      // life time for endpoint's responses (mandatory)
      "expire_in": "60s",
    },
  },
}
```

## Design

See [DESIGN.md](./DESIGN.md) for design document.
