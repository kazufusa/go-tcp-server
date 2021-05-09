package tcpserver

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

var (
	testTimeout = 100 * time.Millisecond
)

func TestConnectionRead(t *testing.T) {
	var message = []byte("test")
	var wg sync.WaitGroup
	wg.Add(2)

	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := Conn{conn: server}
	err := conn.SetReadDeadline(time.Now().Add(testTimeout))
	if err != nil {
		t.Error(err)
	}

	go func() {
		defer wg.Done()
		err = client.SetWriteDeadline(time.Now().Add(testTimeout))
		if err != nil {
			t.Error(err)
		}
		_, err := client.Write(message)
		if err != nil {
			t.Error(err)
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			t.Error(err)
		}
		if n != 4 || !bytes.Equal(message, buf[0:n]) {
			t.Errorf("expected %s, actual %s", message, buf[0:n])
		}
	}()
	wg.Wait()
}

func TestConnectionReadSeparately(t *testing.T) {
	var message = []byte("test")
	var wg sync.WaitGroup
	wg.Add(2)

	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := Conn{conn: server}
	err := conn.SetReadDeadline(time.Now().Add(testTimeout))
	if err != nil {
		t.Error(err)
	}

	go func() {
		defer wg.Done()
		err = client.SetWriteDeadline(time.Now().Add(testTimeout))
		if err != nil {
			t.Error(err)
		}
		_, err := client.Write(message)
		if err != nil {
			t.Error(err)
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 1)
		for i := range message {
			n, err := conn.Read(buf)
			if err != nil {
				t.Error(err)
			}
			if n != 1 || buf[0] != message[i] {
				t.Errorf("expected %q, actual %s", message[i], buf[0:n])
			}
		}
	}()
	wg.Wait()
}

func TestConnectionReadTimeout(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()
	conn := Conn{conn: server}
	err := conn.SetReadDeadline(time.Now())
	if err != nil {
		t.Error(err)
	}

	_, err = conn.Read([]byte{})
	if err == nil {
		t.Error("Read should be timeout error")
	} else if nerr, ok := err.(net.Error); !ok || !nerr.Timeout() {
		t.Errorf("expected timeout error, actual %s", nerr.Error())
	}
}

func TestConnectionWrite(t *testing.T) {
	var message = []byte("test")
	var wg sync.WaitGroup
	wg.Add(2)

	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := Conn{conn: server}
	err := conn.SetWriteDeadline(time.Now().Add(testTimeout))
	if err != nil {
		t.Error(err)
	}

	go func() {
		defer wg.Done()
		err = conn.SetWriteDeadline(time.Now().Add(testTimeout))
		if err != nil {
			t.Error(err)
		}
		_, err := conn.Write(message)
		if err != nil {
			t.Error(err)
		}
	}()

	go func() {
		defer wg.Done()
		err = client.SetReadDeadline(time.Now().Add(testTimeout))
		if err != nil {
			t.Error(err)
		}
		buf := make([]byte, 1024)
		n, err := client.Read(buf)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(buf[0:n], message) {
			t.Errorf("expected %s, actual %s", message, buf)
		}
	}()
	wg.Wait()
}

func TestConnectionWriteTimeout(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()
	conn := Conn{conn: server}
	err := conn.SetWriteDeadline(time.Now())
	if err != nil {
		t.Error(err)
	}

	_, err = conn.Write([]byte{})
	if err == nil {
		t.Error("should be error")
	} else if nerr, ok := err.(net.Error); !ok || !nerr.Timeout() {
		t.Errorf("expected timeout error, actual %s", nerr.Error())
	}
}
