package gotcp

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type Config struct {
	Recover                bool   // print fatal stack, helpfull for debug
	PacketSendChanLimit    uint32 // the limit of packet send channel
	PacketReceiveChanLimit uint32 // the limit of packet receive channel
}

type Server struct {
	config    *Config         // server configuration
	callback  ConnCallback    // message callbacks in connection
	protocol  Protocol        // customize packet protocol
	exitChan  chan struct{}   // notify all goroutines to shutdown
	waitGroup *sync.WaitGroup // wait for all goroutines
}

// NewServer creates a server
func NewServer(config *Config, callback ConnCallback, protocol Protocol) *Server {
	return &Server{
		config:    config,
		callback:  callback,
		protocol:  protocol,
		exitChan:  make(chan struct{}),
		waitGroup: &sync.WaitGroup{},
	}
}

// Start starts service
func (s *Server) Start(listener *net.TCPListener, acceptTimeout time.Duration) {
	s.waitGroup.Add(1)
	defer func() {
		listener.Close()
		s.waitGroup.Done()
	}()

	for {
		select {
		case <-s.exitChan:
			return

		default:
		}

		listener.SetDeadline(time.Now().Add(acceptTimeout))

		conn, err := listener.AcceptTCP()
		if err != nil {
			continue
		}

		s.waitGroup.Add(1)
		go func() {
			newConn(conn, s, conn.RemoteAddr().String()).Do()
			s.waitGroup.Done()
		}()
	}
}

func (s *Server) StartTcp(addr string, acceptTimeout time.Duration) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", addr)
	if err != nil {
		panic(err)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		panic(err)
	}

	s.waitGroup.Add(1)
	defer func() {
		listener.Close()
		s.waitGroup.Done()
	}()

	for {
		select {
		case <-s.exitChan:
			return

		default:
		}

		listener.SetDeadline(time.Now().Add(acceptTimeout))

		conn, err := listener.AcceptTCP()
		if err != nil {
			continue
		}

		s.waitGroup.Add(1)
		go func() {
			newConn(conn, s, conn.RemoteAddr().String()).Do()
			s.waitGroup.Done()
		}()
	}
}

func (s *Server) StartProxyTcp(addr string, acceptTimeout time.Duration) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", addr)
	if err != nil {
		panic(err)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		panic(err)
	}

	proxyListener := &ProxyListener{Listener: listener}

	s.waitGroup.Add(1)
	defer func() {
		proxyListener.Close()
		s.waitGroup.Done()
	}()

	for {
		select {
		case <-s.exitChan:
			return

		default:
		}

		proxyListener.SetDeadline(time.Now().Add(acceptTimeout))

		conn, err := proxyListener.AcceptTCP()
		if err != nil {
			continue
		}

		s.waitGroup.Add(1)
		go func() {
			newConn(conn, s, conn.RemoteAddr().String()).Do()
			s.waitGroup.Done()
		}()
	}
}

func (s *Server) StartWs(addr string, path string) {
	s.waitGroup.Add(1)
	defer func() {
		s.waitGroup.Done()
	}()

	http.Handle(path, websocket.Handler(func(conn *websocket.Conn) {
		conn.PayloadType = websocket.BinaryFrame //这个非常非常重要
		c := newConn(conn, s, conn.Request().RemoteAddr)
		c.Do()
		<-c.closeChan
	}))

	err := http.ListenAndServe(addr, nil)
	if err != nil {
		panic(err)
	}
}

// Stop stops service
func (s *Server) Stop() {
	close(s.exitChan)
	s.waitGroup.Wait()
}
