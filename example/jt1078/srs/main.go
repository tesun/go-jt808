package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"log/slog"
	"os"
	"os/signal"
	"srs/internal"
	"syscall"
)

var global Config

type (
	Config struct {
		Realtime VideoConfig `yaml:"realtime" mapstructure:"realtime"`
		Playback VideoConfig `yaml:"playback" mapstructure:"playback"`
	}

	VideoConfig struct {
		Enable           bool   `yaml:"enable" mapstructure:"enable"`
		Address          string `yaml:"address" mapstructure:"address"`
		SRSWhipURLPrefix string `yaml:"srs_whip_url_prefix" mapstructure:"srs_whip_url_prefix"`
		OnJoinURL        string `yaml:"on_join_url" mapstructure:"on_join_url"`
		OnLeaveURL       string `yaml:"on_leave_url" mapstructure:"on_leave_url"`
	}
)

func init() {
	viper.SetConfigFile("config.yaml")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	if err := viper.Unmarshal(&global); err != nil {
		panic(err)
	}

	b, _ := json.MarshalIndent(global, "", "  ")
	fmt.Println(string(b))

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	}))
	slog.SetDefault(logger)
}

func main() {
	if v := global.Realtime; v.Enable {
		realtime := internal.NewJT1078Service(v.Address, v.SRSWhipURLPrefix, v.OnJoinURL, v.OnLeaveURL)
		go realtime.Run()
	}

	if v := global.Playback; v.Enable {
		playback := internal.NewJT1078Service(v.Address, v.SRSWhipURLPrefix, v.OnJoinURL, v.OnLeaveURL)
		go playback.Run()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	<-sigChan
}
