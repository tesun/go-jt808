package main

import (
	"errors"
	"fmt"
	"github.com/cuteLittleDevil/go-jt808/protocol/model"
	"github.com/cuteLittleDevil/go-jt808/service"
	"time"
)

const (
	serverAddress = "101.35.2.3"
	serverPort    = 1078
)

func main() {
	var goJT808 *service.GoJT808
	goJT808 = service.New(
		service.WithHostPorts("0.0.0.0:808"),
		service.WithCustomTerminalEventer(func() service.TerminalEventer {
			return &meTerminal{
				server: goJT808,
			}
		}),
	)
	goJT808.Run()
}

type meTerminal struct {
	server *service.GoJT808
}

func (m *meTerminal) OnJoinEvent(msg *service.Message, key string, err error) {
	if err == nil {
		fmt.Println("加入", key)
		go func(sim string) {
			time.Sleep(1 * time.Second)
			p9101 := model.P0x9101{
				ServerIPLen:  byte(len(serverAddress)),
				ServerIPAddr: serverAddress,
				TcpPort:      serverPort,
				UdpPort:      0,
				ChannelNo:    1,
				DataType:     1, //  0-音视频 1-视频 2-双向对讲 3-监听 4-中心广播 5-透传
				StreamType:   0,
			}
			fmt.Println(key, p9101.String())
			result := m.server.SendActiveMessage(&service.ActiveMessage{
				Key:              sim,
				Command:          p9101.Protocol(),
				Body:             p9101.Encode(),
				OverTimeDuration: 3 * time.Second,
			})

			if err := result.ExtensionFields.Err; err == nil {
				fmt.Println("结果", result.JTMessage.Header.String())
			} else if errors.Is(err, service.ErrWriteDataOverTime) {
				fmt.Println("超时", err)
			} else {
				fmt.Println("其他异常", err)
			}
		}(key)

	}
}

func (m *meTerminal) OnLeaveEvent(key string) {

}

func (m *meTerminal) OnNotSupportedEvent(msg *service.Message) {
}

func (m *meTerminal) OnReadExecutionEvent(msg *service.Message) {

}

func (m *meTerminal) OnWriteExecutionEvent(msg service.Message) {
}
