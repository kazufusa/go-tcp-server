package tcpserver

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	nextPollInterval = 50 * time.Millisecond
)

type TcpServer struct {
	l          net.Listener
	inShutdown int32
	handler    Handler
	mu         sync.Mutex
	activeConn map[*net.Conn]struct{}
}

type Handler interface {
	ServeRequest(conn *Conn)
}

func New(
	address string,
	handler Handler,
) (*TcpServer, error) {
	tcpAddr, err := net.ResolveTCPAddr(
		"tcp",
		address,
	)
	if err != nil {
		return nil, err
	}
	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
	}

	return &TcpServer{l: l, handler: handler}, nil
}

func (t *TcpServer) ListenAndServe() error {
	for {
		conn, err := t.l.Accept()
		if err != nil {
			if nerr, ok := err.(*net.OpError); ok {
				if nerr.Temporary() {
					continue
				} else if nerr.Err.Error() == "use of closed network connection" {
					return nil
				}
			}
			return err
		}
		if t.shuttingDown() {
			return nil
		} else {
			t.trackConn(&conn, true)
			go func() {
				defer func() {
					_ = recover()
					_ = conn.Close()
					t.trackConn(&conn, false)
				}()
				t.handler.ServeRequest(&Conn{conn: conn})
			}()
		}
	}
}

func (t *TcpServer) Shutdown(ctx context.Context) error {
	atomic.StoreInt32(&t.inShutdown, 1)

	ticker := time.NewTicker(nextPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = t.l.Close()
			t.closeAllActiveConns()
			return ctx.Err()
		case <-ticker.C:
			if t.allConsClosed() {
				return t.l.Close()
			}
		}
	}
}

func (t *TcpServer) trackConn(c *net.Conn, add bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.activeConn == nil {
		t.activeConn = make(map[*net.Conn]struct{})
	}
	if add {
		t.activeConn[c] = struct{}{}
	} else {
		delete(t.activeConn, c)
	}
}

func (t *TcpServer) allConsClosed() bool {
	return len(t.activeConn) == 0
}

func (t *TcpServer) closeAllActiveConns() {
	for c := range t.activeConn {
		_ = (*c).Close()
		delete(t.activeConn, c)
	}
}

func (t *TcpServer) shuttingDown() bool {
	return atomic.LoadInt32(&t.inShutdown) != 0
}

type Conn struct {
	conn net.Conn
}

func (c *Conn) SetDeadline(deadline time.Time) error {
	return c.conn.SetDeadline(deadline)
}

func (c *Conn) SetReadDeadline(deadline time.Time) error {
	return c.conn.SetReadDeadline(deadline)
}

func (c *Conn) SetWriteDeadline(deadline time.Time) error {
	return c.conn.SetWriteDeadline(deadline)
}

func (c *Conn) Read(buf []byte) (n int, err error) {
	for {
		n, err = c.conn.Read(buf)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() && !nerr.Timeout() {
				continue
			}
			return
		}
		return
	}
}

func (c *Conn) Write(res []byte) (n int, err error) {
	for {
		n, err = c.conn.Write(res)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() && !nerr.Timeout() {
				res = res[n:]
				continue
			}
			return
		}
		return
	}
}
