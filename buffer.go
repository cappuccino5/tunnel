package main

import (
	"dev.risinghf.com/go/framework/log"
	"sync"
)

const BufferSize = 2048

type Payload struct {
	Data []byte
}

// pool 实际数据缓冲区，缓冲区的容量由 golang 自动控制，PayloadIn 等通道只是个内存地址列表
var pool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, BufferSize)
		pl := Payload{
			Data: b,
		}
		return &pl
	},
}

func getPayloadBuffer() *Payload {
	pl := pool.Get().(*Payload)
	return pl
}

func putPayloadBuffer(pl *Payload) {
	// DPD-REQ、KEEPALIVE 等数据
	if cap(pl.Data) != BufferSize {
		log.Debug("payload is:", pl.Data)
		return
	}

	pl.Data = pl.Data[:BufferSize]
	pool.Put(pl)
}
