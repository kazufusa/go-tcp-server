package tcpserver_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	tcpserver "github.com/kazufusa/go-tcp-server"
)

var (
	testMessage = []byte("hello")
	testTimeout = 100 * time.Millisecond
)

type echoHandler struct {
	t             *testing.T
	startServe    chan<- struct{}
	writeResponse <-chan struct{}
}

var _ tcpserver.Handler = (*echoHandler)(nil)

func (h *echoHandler) ServeRequest(conn *tcpserver.Conn) {
	if h.startServe != nil {
		h.startServe <- struct{}{}
	}
	buf := make([]byte, 1024)
	nReq, err := conn.Read(buf)
	if err != nil {
		h.t.Error(err)
	}
	if h.writeResponse != nil {
		<-h.writeResponse
	}
	nRes, err := conn.Write(buf[0:nReq])
	if err != nil {
		h.t.Error(err)
	}
	if nReq != nRes {
		h.t.Error("failed to echo a message completely")
	}
}

func makeTestServer(t *testing.T) (*tcpserver.TCPServer, *net.TCPAddr, <-chan struct{}, chan<- struct{}, func(), error) {
	var port string = "8080"
	if v, ok := os.LookupEnv("TEST_PORT"); ok {
		port = v
	}
	startServe := make(chan struct{}, 1)
	writeResponse := make(chan struct{}, 1)
	h := echoHandler{startServe: startServe, writeResponse: writeResponse}
	server, err := tcpserver.New(fmt.Sprintf(":%s", port), &h)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%s", port))
	return server, tcpAddr, startServe, writeResponse, func() { close(startServe); close(writeResponse) }, err
}

func clientTest(t *testing.T, tcpAddr *net.TCPAddr, wg *sync.WaitGroup, eof bool) {
	defer wg.Done()
	client, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		t.Error(err)
	}
	_, err = client.Write(testMessage)
	if err != nil {
		t.Error(err)
	}
	buf := make([]byte, 1024)
	n, err := client.Read(buf)
	if eof {
		if err != io.EOF {
			t.Fatalf("expected %s, actual %s", io.EOF, err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	ret := buf[0:n]
	if !bytes.Equal(testMessage, ret) {
		t.Errorf(`expected "%s", actual "%s"`, testMessage, ret)
	}
	err = client.Close()
	if err != nil {
		t.Error(err)
	}
}

func TestTcpServer(t *testing.T) {
	var wg sync.WaitGroup
	server, tcpAddr, startServe, writeResponse, cleanUp, err := makeTestServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanUp()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != tcpserver.ErrServerClosed {
			t.Error(err)
		}
	}()

	go func() {
		<-startServe
		writeResponse <- struct{}{}
	}()

	wg.Add(1)
	go clientTest(t, tcpAddr, &wg, false)
	wg.Wait()

	err = server.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}
}

func TestTcpServerShutdown(t *testing.T) {
	server, _, _, _, cleanup, err := makeTestServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != tcpserver.ErrServerClosed {
			t.Error(err)
		}
	}()

	err = server.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}
}

func TestTcpServerShutdownWithBlocking(t *testing.T) {
	var wg sync.WaitGroup
	server, tcpAddr, startServe, writeResponse, cleanUp, err := makeTestServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanUp()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != tcpserver.ErrServerClosed {
			t.Error(err)
		}
	}()

	wg.Add(1)
	go clientTest(t, tcpAddr, &wg, false)

	<-startServe
	go func() {
		time.Sleep(testTimeout)
		writeResponse <- struct{}{}
	}()
	err = server.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}
	wg.Wait()
}

func TestTcpServerShutdownTimedOut(t *testing.T) {
	var wg sync.WaitGroup
	server, tcpAddr, startServe, _, cleanUp, err := makeTestServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanUp()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != tcpserver.ErrServerClosed {
			t.Error(err)
		}
	}()

	wg.Add(1)
	go clientTest(t, tcpAddr, &wg, true)

	<-startServe
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	err = server.Shutdown(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected %v, actual %v", context.DeadlineExceeded, err)
	}
	err = server.Close()
	if err != nil {
		t.Error(err)
	}
	wg.Wait()
}

func TestTcpServerFailToRestart(t *testing.T) {
	server, _, _, _, cleanUp, err := makeTestServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanUp()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != tcpserver.ErrServerClosed {
			t.Error(err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	if err = server.Shutdown(ctx); err != nil {
		t.Error(err)
	}
	if err = server.ListenAndServe(); err != tcpserver.ErrServerClosed {
		t.Errorf("expected %#v, actual %#v", tcpserver.ErrServerClosed, err)
	}
}
