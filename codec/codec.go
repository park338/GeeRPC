package codec

import "io"

type Header struct {
	ServiceMethod string //请求方法
	Seq           uint64 //请求序号
	Error         string //错误信息
}

type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

type NewCodeFunc func(io.ReadWriteCloser) Codec

type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
)

var NewCodeFuncMap map[Type]NewCodeFunc

func init() {
	NewCodeFuncMap = make(map[Type]NewCodeFunc)
	NewCodeFuncMap[GobType] = NewGobCodec
}
