package tcpserver

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// shutdownPollInterval is the polling interval when checking
// quiescence during TcpServer.Shutdown.
const shutdownPollInterval = 50 * time.Millisecond

// ErrServerClosed is returned by the TCPServer'sListenAndServe after
// a call to Shutdown or Close.
var ErrServerClosed = errors.New("use of closed network connection")

// A TCPServer defines parameters for running an TCP server.
type TCPServer struct {
	// l is a network listener.
	l *net.TCPListener

	// true when server shutdown
	inShutdown int32
	handler    Handler
	mu         sync.Mutex
	activeConn map[*net.Conn]struct{}
}

// A Handler reads and responds to an client connection.
type Handler interface {
	ServeRequest(conn *Conn)
}

// New returns a new TCPServer.
func New(
	address string,
	handler Handler,
) (*TCPServer, error) {
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

	return &TCPServer{l: l, handler: handler}, nil
}

// ListenAndServe listens TCPListener srv.l and then
// calls ServeRequest to handle requests on incoming connections.
func (srv *TCPServer) ListenAndServe() error {
	for {
		conn, err := srv.l.Accept()
		if err != nil {
			if nerr, ok := err.(*net.OpError); ok {
				if nerr.Temporary() {
					continue
				} else if nerr.Err.Error() == ErrServerClosed.Error() {
					return ErrServerClosed
				}
			}
			return err
		}
		if srv.shuttingDown() {
			return nil
		} else {
			srv.trackConn(&conn, true)
			go func() {
				defer func() {
					_ = recover()
					_ = conn.Close()
					srv.trackConn(&conn, false)
				}()
				srv.handler.ServeRequest(&Conn{conn: conn})
			}()
		}
	}
}

// Shutdown gracefully shuts down the server without interrupting any
// active connections. Shutdown waits indefinitely for all connections
// closed. If the provided context expires before the shutdown is
// complete, Shutdown returns the context's error, otherwise it returns
// any error returned from closing the Server's underlying Listener srv.l.
//
// Once Shutdown has been called on a server, it may not be reused;
// future calls to methods such as Serve will return ErrServerClosed.
func (srv *TCPServer) Shutdown(ctx context.Context) error {
	atomic.StoreInt32(&srv.inShutdown, 1)

	ticker := time.NewTicker(shutdownPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if srv.allConsClosed() {
				return srv.l.Close()
			}
		}
	}
}

// Close immediately closes a net.Listener srv.l and any connections.
// For agraceful shutdown, use Shutdown.
//
// Close returns an error returned from closing the Server's
// net.Listener srv.l.
func (srv *TCPServer) Close() error {
	atomic.StoreInt32(&srv.inShutdown, 1)
	srv.mu.Lock()
	defer srv.mu.Unlock()

	for c := range srv.activeConn {
		_ = (*c).Close()
		delete(srv.activeConn, c)
	}
	return srv.l.Close()
}

func (srv *TCPServer) trackConn(c *net.Conn, add bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.activeConn == nil {
		srv.activeConn = make(map[*net.Conn]struct{})
	}
	if add {
		srv.activeConn[c] = struct{}{}
	} else {
		delete(srv.activeConn, c)
	}
}

func (srv *TCPServer) allConsClosed() bool {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return len(srv.activeConn) == 0
}

func (srv *TCPServer) shuttingDown() bool {
	return atomic.LoadInt32(&srv.inShutdown) != 0
}

// Conn is a wrapper of net.Conn and handles net.Error.Temporary()
// and net.Error.Timeout() in Read/Write.
type Conn struct {
	conn net.Conn
}

// SetDeadline sets the read and write deadlines associated
// with the connection with net.Conn.SetDeadline.
func (c *Conn) SetDeadline(deadline time.Time) error {
	return c.conn.SetDeadline(deadline)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call with net.Conn.SetReadDeadline.
func (c *Conn) SetReadDeadline(deadline time.Time) error {
	return c.conn.SetReadDeadline(deadline)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call with net.Conn.SetWriteDeadline.
func (c *Conn) SetWriteDeadline(deadline time.Time) error {
	return c.conn.SetWriteDeadline(deadline)
}

// Read is a wrapper of net.Conn.Read and reads data from the connection.
// If net.Conn.Read returns an Error with Temporary() == true,
// Read retries to read.
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

// Write is a wrapper of net.Conn.Write and writes data to the connection.
// If net.Conn.Write returns an Error with Temporary() == true,
// Write retries to write.
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
