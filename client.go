package main

import (
	"github.com/smallnest/ringbuffer"
)

type connection struct {
	buf           *ringbuffer.RingBuffer
	closeCallback func(err error)
	parseCallBack func(raw []byte)
}

func (c *connection) Start() {
	r := NewRESPReader(c.buf, 1024*1024)

	for {
		raw, err := r.ReadRaw()
		if err != nil {
			c.closeCallback(err)
			return
		}
		c.parseCallBack(raw)
	}
}
