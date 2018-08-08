package signal

import (
	"os"
	"reflect"
)

type SignalSet struct {
	handlers map[os.Signal]reflect.Value
}

// NewSignalSet 新建一个signalset并返回
func NewSignalSet() *SignalSet {
	ss := new(SignalSet)
	ss.handlers = make(map[os.Signal]reflect.Value)
	return ss
}

// Register 注册某个signal的处理函数，handler应该是一个可执行的函数
func (ss *SignalSet) Register(sig os.Signal, handler interface{}) {
	if _, found := ss.handlers[sig]; !found {
		ss.handlers[sig] = reflect.ValueOf(handler)
	}
}

// Handle 处理接收到的信号
func (ss *SignalSet) Handle(sig os.Signal, args ...interface{}) {
	if _, found := ss.handlers[sig]; found {
		var callArgsv []reflect.Value
		for _, arg := range args {
			callArgsv = append(callArgsv, reflect.ValueOf(arg))
		}
		ss.handlers[sig].Call(callArgsv)
	}
}
