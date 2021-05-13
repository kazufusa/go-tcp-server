# TCP Server with Golang

## Example

This is a TCP echo server that can shutdown gracefully.

```sh
$ go run ./cmd/main.go
Listening on localhost:8080, CTRL-C to stop server
^C
2021/05/13 12:54:57 shutdown server...
2021/05/13 12:55:07 shutdown failed: context deadline exceeded
```

```sh
$ nc localhost 8080
aaa
aaa
bbb
bbb
ccc
ccc


exit
exit
```
