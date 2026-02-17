package internal

import (
	"errors"
	"fmt"
	"github.com/cuteLittleDevil/go-jt808/protocol/jt1078"
	"io"
	"log/slog"
	"net"
)

type connection struct {
	conn             net.Conn
	stopChan         chan struct{}
	onJoinEvent      func(c *connection, pack *jt1078.Packet)
	onLeaveEvent     func()
	srsWhipURLPrefix string
}

func newConnection(c net.Conn, srsWhipURLPrefix string) *connection {
	return &connection{
		conn:             c,
		stopChan:         make(chan struct{}),
		srsWhipURLPrefix: srsWhipURLPrefix,
	}
}

func (c *connection) run() error {
	var (
		data       = make([]byte, 10*1024)
		packParse  = newPackageParse()
		whipHandle *whip
	)
	defer func() {
		packParse.clear()
		clear(data)
		c.stop()
		whipHandle.stop()
	}()

	for {
		if n, err := c.conn.Read(data); err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return nil
			}
			return err
		} else if n > 0 {
			var packs []*jt1078.Packet
			for pack, err := range packParse.parse(data[:n]) {
				if err == nil {
					packs = append(packs, pack)
				} else if errors.Is(err, jt1078.ErrBodyLength2Short) || errors.Is(err, jt1078.ErrHeaderLength2Short) {
					// 数据长度不够的 忽略
				} else {
					return err
				}
			}

			for _, v := range packs {
				if whipHandle == nil {
					if v.PT == jt1078.PTH264 || v.PT == jt1078.PTH265 {
						url := fmt.Sprintf("%s-%s-%d", c.srsWhipURLPrefix, v.Sim, v.LogicChannel)
						whipHandle = newWhip(url)
						if err = whipHandle.run(v); err != nil {
							slog.Error("whip fail",
								slog.String("sim", v.Sim),
								slog.Any("err", err))
							return err
						}
					}
				} else {
					whipHandle.sendData(v)
				}
			}
		}

	}
}

func (c *connection) stop() {
	close(c.stopChan)
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.onLeaveEvent()
}
