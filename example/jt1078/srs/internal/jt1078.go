package internal

import (
	"context"
	"github.com/cuteLittleDevil/go-jt808/protocol/jt1078"
	"log/slog"
	"net"
	"time"
)

type JT1078Service struct {
	address       string
	whipURLPrefix string
	onJoinURL     string
	onLeaveURL    string
}

func NewJT1078Service(address string, whipURLPrefix string, onJoinURL string, onLeaveURL string) *JT1078Service {
	return &JT1078Service{address: address, whipURLPrefix: whipURLPrefix, onJoinURL: onJoinURL, onLeaveURL: onLeaveURL}
}

func (j *JT1078Service) Run() {
	listen, err := net.Listen("tcp", j.address)
	if err != nil {
		slog.Error("listen error",
			slog.String("address", j.address),
			slog.Any("err", err))
		return
	}
	slog.Info("listen tcp",
		slog.String("address", j.address),
		slog.String("join", j.onJoinURL),
		slog.String("leave", j.onLeaveURL))
	for {
		conn, err := listen.Accept()
		if err != nil {
			slog.Warn("accept error",
				slog.String("err", err.Error()))
			return
		}
		client := newConnection(conn, j.whipURLPrefix)
		httpBody := map[string]any{}
		ctx, cancel := context.WithCancel(context.Background())
		client.onJoinEvent = func(c *connection, pack *jt1078.Packet) {
			httpBody = map[string]any{
				"sim":       pack.Sim,
				"channel":   pack.LogicChannel,
				"startTime": time.Now().Format(time.DateTime),
			}
			onNoticeEvent(j.onJoinURL, httpBody)
		}

		client.onLeaveEvent = func() {
			if len(httpBody) > 0 {
				httpBody["endTime"] = time.Now().Format(time.DateTime)
				onNoticeEvent(j.onLeaveURL, httpBody)
			}
			cancel()
		}
		go func(ctx context.Context) {
			if err := client.run(); err != nil {
				slog.Warn("run error",
					slog.Any("http body", httpBody),
					slog.String("err", err.Error()))
			}
		}(ctx)
	}
}
