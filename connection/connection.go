package connection

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/iamwwc/tlsmiddleman/common"
	"github.com/iamwwc/tlsmiddleman/decoder"
	"io"
	"net"
	"net/http"
	"time"
)

func NewConnectionHandler(w http.ResponseWriter, r *http.Request, interceptor *Interceptor, conn *net.Conn) *Handler {
	return &Handler{
		interceptor,
		*conn,
		w,
		r,
	}
}

type Handler struct {
	interceptor *Interceptor
	conn        net.Conn
	response    http.ResponseWriter
	request     *http.Request
}

func (this *Handler) TLSHandshake() {
	tlsConfig := decoder.NewDefaultServerTlsConfig()
	cert, err := this.interceptor.CA.Sign(this.request.Host)
	if err != nil {
		fmt.Println(err)
		return
	}
	tlsCert, err := this.interceptor.CA.ToTLSCertificate(cert)
	if err != nil {
		fmt.Println(err)
		return
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConn := tls.Server(this.conn, tlsConfig)
	this.conn = tlsConn
	tlsConn.Handshake()
	go this.Pipe()
}

// Pipe处理裸HTTP，并向remote转发数据
// 这里需要区分 src 是HTTPS还是HTTP，对于HTTPS需要作为TLS client和remote连接
// 如果是HTTP直接转发就行
// ResponseWriter 和 Request 都放在 this 上
func (this *Handler) Pipe() {
	remote := <- this.connectToRemote()
	if remote == nil {
		fmt.Println("Connect to remote failed, return")
		return
	}
	chan1 := common.ChannelFromConn(this.conn)
	chan2 := common.ChannelFromConn(remote)
	defer func() {
		remote.Close()
		this.conn.Close()
	}()
	for {
		select {
		case b1 := <-chan1:
			if b1 == nil {
				return
			}
			remote.Write(b1)
		case b2 := <- chan2:
			if b2 == nil {
				return
			}
			this.conn.Write(b2)
		}
	}
}

func (this Handler) connectToRemote() <- chan net.Conn{
	c := make(chan net.Conn,1)
	go func() {
		target := this.request.URL.Host
		port := this.request.URL.Port()
		var conn net.Conn
		var err error
		if port == "443" {
			conn, err = tls.Dial("tcp",target, decoder.NewDefaultServerTlsConfig())
			if err != nil {
				fmt.Println(err)
				c <- nil
				return
			}
		}else {
			conn, err = net.DialTimeout("tcp",target, time.Second * 60)
			if err != nil {
				fmt.Println(err)
				c <- nil
				return
			}
		}
		c <- conn
	}()
	return c
}

func (this *Handler) dumpHTTPRequest(request *common.ReaderHelper)  {
	req, err := http.ReadRequest(bufio.NewReader(request))
	if err != nil {
		return
	}
	fmt.Println(req.Host)
}

func (this *Handler) Accept() (net.Conn, error) {
	if this.conn != nil {
		c := this.conn
		this.conn = nil
		return c, nil
	}
	return nil, io.EOF
}

func (this *Handler) Close() error {
	return this.conn.Close()
}

func (this *Handler) Addr() net.Addr {
	return nil
}