# apcupsc - a package for reading apcupsd status

## Overview

The `apcupsc` package provides a [Go
API](https://pkg.go.dev/zappem.net/pub/net/apcupsc) for finding and
reading the status of
[`apcupsd`](https://en.wikipedia.org/wiki/Apcupsd) services over the
network.

You can get a sample of its functionality using the included example:

```
$ git clone https://github.com/tinkerator/apcupsc.git
$ cd apcupsc
$ go run examples/apcupsc.go
2024/10/19 11:46:33 localhost:3551: &apcupsc.Target{Power:45, Charge:77, Backup:103, Charged:true, Offline:false, Name:"myapc", LineV:120, XFers:1, LastOnBattery:time.Date(2024, time.October, 3, 3, 11, 10, 0, time.Local), LastOutage:"2024-10-03 03:11:10 -0700", Lasted:2000000000, Duration:"2s"}
```

## License info

The `apcupsc` package is distributed with the same BSD 3-clause
license as that used by [golang](https://golang.org/LICENSE) itself.

## Reporting bugs and feature requests

The `apcupsc` package has been developed purely out of self-interest
If you find a bug or want to suggest a feature addition, please use
the [bug tracker](https://github.com/tinkerator/apcupsc/issues).
