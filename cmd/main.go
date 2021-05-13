package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	tcpserver "github.com/kazufusa/go-tcp-server"
)

type echoHandler struct{}

var _ tcpserver.Handler = (*echoHandler)(nil)

func (h *echoHandler) ServeRequest(conn *tcpserver.Conn) {
	buf := make([]byte, 1024)
	for {
		conn.SetReadDeadline(time.Now().Add(time.Second * 10))
		conn.SetWriteDeadline(time.Now().Add(time.Second * 10))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		_, err = conn.Write(buf[0:n])
		if err != nil {
			return
		}
	}
}

func main() {
	srv, err := tcpserver.New(":8080", &echoHandler{})
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != tcpserver.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	fmt.Println("Listening on localhost:8080, CTRL-C to stop server")

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	fmt.Println("")
	log.Println("shutdown server...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	if err = srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown failed: %s", err)
		err = srv.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
}
