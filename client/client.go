package client

import (
	"bufio"
	GeeRPC "codec"
	"codec/codec"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	connected        = "200 Connected to Gee RPC"
	defaultRPCPath   = "/_geeprc_"
	defaultDebugPath = "/debug/geerpc"
)

// 单个请求call
type Call struct {
	Seq           uint64      //请求编号
	ServiceMethod string      //方法名+服务名
	Args          interface{} //请求参数
	Reply         interface{} //返回参数
	Error         error       //错误提示
	Done          chan *Call  //调用结束通道
}

func (call *Call) done() {
	call.Done <- call
}

// 客户端
type Client struct {
	cc       codec.Codec    //处理信息
	opt      *GeeRPC.Option //证书
	sending  sync.Mutex     //发送锁
	header   codec.Header   //请求头
	mu       sync.Mutex
	seq      uint64           //客户端编号
	pending  map[uint64]*Call //存储请求call
	closing  bool             //是否关闭客户端
	shutdown bool             //客户端异常关闭
}
type clientResult struct {
	client *Client
	err    error
}

// 函数模板
type newClientFunc func(conn net.Conn, opt *GeeRPC.Option) (client *Client, err error)

var _ io.Closer = (*Client)(nil)

var ErrShutdown = errors.New("connection is shut down")

// 关闭客户端
func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

// 判断客户端是否可用
func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing
}

// 注册call
func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

// 获取并移除请求call
func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

// 异常call结果
func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

// 客户端接收消息
func (client *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil {
			break
		}
		call := client.removeCall(h.Seq)
		switch {
		case call == nil:
			err = client.cc.ReadBody(nil)
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}

	}
	client.terminateCalls(err)
}

// tcp初始化client
func NewClient(conn net.Conn, opt *GeeRPC.Option) (*Client, error) {
	f := codec.NewCodeFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("不合法的CodecType %s", opt.CodecType)
		log.Println("rpc客户端:codec错误:", err)
		return nil, err
	}
	if err := json.NewDecoder(conn).Decode(opt); err != nil {
		log.Println("rpc客户端：opt错误 ", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(cc codec.Codec, opt *GeeRPC.Option) *Client {
	client := &Client{
		seq:     1, // 编号初始值为1
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

// option初始化
func parseOptions(opts ...*GeeRPC.Option) (*GeeRPC.Option, error) {
	// if opts is nil or pass nil as parameter
	if len(opts) == 0 || opts[0] == nil {
		return GeeRPC.DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = GeeRPC.DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = GeeRPC.DefaultOption.CodecType
	}
	return opt, nil
}

// 普通tcp连接服务端
func dialTimeout(f newClientFunc, network, address string, opts ...*GeeRPC.Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	//关闭conn通道
	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()
	ch := make(chan clientResult)
	go func() {
		client, err := f(conn, opt)
		ch <- clientResult{client: client, err: err}
	}()
	if opt.ConnectTimeout == 0 {
		result := <-ch
		return result.client, result.err
	}
	select {
	case <-time.After(opt.ConnectTimeout):
		return nil, fmt.Errorf("rpc客户端连接超时 %s", opt.ConnectTimeout)
	case result := <-ch:
		return result.client, result.err
	}

}

// http连接初始化
func NewHTTPClient(conn net.Conn, opt *GeeRPC.Option) (*Client, error) {
	_, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", defaultRPCPath))
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connected {
		return NewClient(conn, opt)
	}
	if err == nil {
		err = errors.New("HTTP 响应错误" + resp.Status)
	}
	return nil, err
}

// http连接服务端
func DialHTTP(network, address string, opts ...*GeeRPC.Option) (*Client, error) {
	return dialTimeout(NewHTTPClient, network, address, opts...)
}

// 普通tcp连接服务端
func Dial(network, address string, opts ...*GeeRPC.Option) (client *Client, err error) {
	return dialTimeout(NewClient, network, address, opts...)
}

// 统一连接函数
func XDial(rpcAddr string, opts ...*GeeRPC.Option) (*Client, error) {
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc 地址名格式错误%s", rpcAddr)
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		return Dial(protocol, addr, opts...)
	}
}

// 发送请求
func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

	//客户端注册call
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	//配置请求头
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	//发送消息
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		call := client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc客户端: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	select {
	case <-ctx.Done():
		client.removeCall(call.Seq)
		return errors.New("客户端调用方法超时" + ctx.Err().Error())
	case call := <-call.Done:
		return call.Error
	}

	return call.Error
}
