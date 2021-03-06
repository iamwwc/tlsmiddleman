package replicant

import (
	"bufio"
	"fmt"
	"github.com/iamwwc/tlsmiddleman/common"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/http/httputil"
)

// Dump也可以使用DumpRequest将Request对象转换成[]byte
// TLS握手结束就http.Server来处理裸HTTP
// 利用现成的*http.Request和http.ResponseWriter配置DumpResponse，DumpRequest搞
// https://golang.org/pkg/net/http/httputil/#DumpRequest
func Dump() (requestChan chan []byte, responseChan chan []byte) {
	requestChan = make(chan []byte, 100)
	reqC := make(chan *http.Request, 100)
	// 这里有问题，写到这里的数据没有返回，导致client发的数据没有写入remote
	go func() {
		for {
			reader := common.NewReaderHelper(requestChan)
			req, err := http.ReadRequest(bufio.NewReader(reader))
			reqC <- req
			if err != nil {
				return
			}
			fmt.Printf("Request: Header: %s\n", req.Header)
		}
	}()
	responseChan = make(chan []byte, 100)
	go func() {
		for {
			reader := common.NewReaderHelper(responseChan)
			req := <-reqC
			// malformed-http-status-code-error 有可能是由HTTP2导致
			// HTTP2数据并不会保证头部先来
			// 需要使用http2包处理
			// 即使这里出错但由于数据并行处理，数据还是能正常传输
			resp, err := http.ReadResponse(bufio.NewReader(reader), req)
			if err != nil {
				logrus.Errorln(err)
				return
			}
			fmt.Printf("Response: Header: %s", resp.Header)
		}
	}()
	return
}

func DumpRequest(r *http.Request) ([]byte, error) {
	return httputil.DumpRequest(r, true)
}

func NewResponseFrom(conn net.Conn, r *http.Request) (*http.Response, error) {
	return http.ReadResponse(bufio.NewReader(conn), r)
}

// 也可以用httputil下的Dump调用来搞
func DumpResponse(w *http.Response) ([]byte, error) {
	return httputil.DumpResponse(w, true)
}
