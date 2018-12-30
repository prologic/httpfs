# httpfs

[![Build](https://cloud.drone.io/api/badges/prologic/httpfs/status.svg)](https://cloud.drone.io/prologic/httpfs)
[![Coverage](https://codecov.io/gh/prologic/httpfs/branch/master/graph/badge.svg)](https://codecov.io/gh/prologic/httpfs)
[![Quality](https://goreportcard.com/badge/github.com/prologic/httpfs)](https://goreportcard.com/report/github.com/prologic/httpfs)
[![Docs](https://godoc.org/github.com/prologic/httpfs?status.svg)](https://godoc.org/github.com/prologic/httpfs)

httpfs is a cross-platform HTTP backed File System implemented using FUSE.
It provides a backend API over HTTP/HTTPS to provide most POSIX file system
calls for a FUSE frontend that presents this API as what looks and feels like
a regular file system.

## Installation

### Source

```#!bash
$ go install github.com/prologic/httpfs/...
```

### OS X Homebrew

```#!bash
$ brew tap prologic/httpfs
$ brew install --HEAD httpfs
```

httpfs is still early days so contributions, ideas and expertise are
much appreciated and highly welome!

## Other Platforms

Please note that at this time (30th November 2016) httpfs is only supported
and tested on Mac OS X with Homebrew installed Go 1.7 and a recent version of
bazil.org/fuse

In theory it should be possible to build httpfs for other platforms as long
as you meet the following requirements:

- Go 1.7+
- FUSE / OSXFUSE

## Usage

Spin up the backend:

```#!bash
$ httpfs -root /path/to/dir
```

Mount it:
```#!bash
$ httpfsmount -url http://localhost:8000 -mount /path/to/mountpoint
```

Then use /path/to/mountpoint as a regular file system!

## Licnese

MIT
