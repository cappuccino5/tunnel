package models

import (
	"sync"
)

const BufferSize = 2048

// pool 实际数据缓冲区，缓冲区的容量由 golang 自动控制，PayloadIn 等通道只是个内存地址列表
var pool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, BufferSize)
		pl := Payload{
			PType: 0x00,
			Data:  b,
		}
		return &pl
	},
}

func GetPayloadBuffer() *Payload {
	pl := pool.Get().(*Payload)
	return pl
}

func PutPayloadBuffer(pl *Payload) {
	// DPD-REQ、KEEPALIVE 等数据
	if cap(pl.Data) != BufferSize {
		// base.Debug("payload is:", pl.Data)
		return
	}
	
	pl.PType = 0x00
	pl.Data = pl.Data[:BufferSize]
	pool.Put(pl)
}
