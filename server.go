package GeeRPC

import (
	"codec/codec"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber int        //辨别rpc请求
	CodecType   codec.Type //编解码的类型
}

// 默认规则
var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

// 默认服务
var DefaultServer = NewServer()

func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc服务接收错误", err)
			return
		}
		go server.ServeConn(conn)
	}
}

// 通过默认的Server提供服务接收通道
func Accept(lis net.Listener) { DefaultServer.Accept(lis) }

func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	f := codec.NewCodeFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	server.serveCodec(f(conn))
}

var invalidRequest = struct{}{}

func (server *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex) //保证完整响应
	wg := new(sync.WaitGroup)  //等待所有响应结束
	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			server.sendResopnse(cc, req.h, invalidRequest, sending)
		}
		wg.Add(1)
		server.handlerRequest(cc, req, sending, wg)
	}
	wg.Wait()
	_ = cc.Close()
}

type request struct {
	h            *codec.Header //请求头
	argv, replyv reflect.Value
}

// 读取请求头
func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: 读取请求头失败:", err)
		}
		return nil, err
	}
	return &h, nil
}

// 读取请求
func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server:读取argv错误", err)
	}
	return req, nil
}

// 发送请求
func (server *Server) sendResopnse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer func() { sending.Unlock() }()
	//TODO不知道argv的类型
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: 响应失败:", err)
	}
}

// 处理请求
func (server *Server) handlerRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	//TODO需要正确的rpc服务获取replyv
	defer wg.Done()
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq))
	server.sendResopnse(cc, req.h, req.replyv.Interface(), sending)
}
