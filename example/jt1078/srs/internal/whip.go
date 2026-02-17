package internal

import (
	"bytes"
	"fmt"
	"github.com/cuteLittleDevil/go-jt808/protocol/jt1078"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type whip struct {
	pc        *webrtc.PeerConnection
	whipURL   string
	videoChan chan *jt1078.Packet
	audioChan chan *jt1078.Packet
}

func newWhip(whipURL string) *whip {
	return &whip{
		whipURL:   whipURL,
		videoChan: make(chan *jt1078.Packet, 100),
		audioChan: nil,
	}
}

func (w *whip) run(packet *jt1078.Packet) error {
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return err
	}

	videoFormat := webrtc.MimeTypeH264
	if packet.PT == jt1078.PTH265 {
		videoFormat = webrtc.MimeTypeH265
	}
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: videoFormat},
		"video", fmt.Sprintf("pion-whip-%s", packet.Sim),
	)
	if err != nil {
		return err
	}

	if _, err = pc.AddTrack(track); err != nil {
		return err
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		return err
	}

	// 等待 ICE gathering 完成
	<-webrtc.GatheringCompletePromise(pc)

	// 发送 Offer
	offerSDP := []byte(offer.SDP)
	req, err := http.NewRequest(http.MethodPost, w.whipURL, bytes.NewBuffer(offerSDP))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/sdp")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return err
	}

	// 处理 Answer
	answerBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  string(answerBytes),
	}

	if err = pc.SetRemoteDescription(answer); err != nil {
		return err
	}

	w.pc = pc
	go w.pumpPacketsToTrack(track, w.videoChan)

	return nil
}

func (w *whip) stop() {
	if w.pc != nil {
		_ = w.pc.Close()
	}
	close(w.videoChan)
	if w.audioChan != nil {
		close(w.audioChan)
	}
}

func (w *whip) sendData(packet *jt1078.Packet) {
	switch packet.PT {
	case jt1078.PTH264, jt1078.PTH265:
		w.videoChan <- packet
	case jt1078.PTG711A, jt1078.PTG711U, jt1078.PTAAC:
		if w.audioChan == nil {
			if err := w.addAudioTrack(packet); err != nil {
				slog.Warn("addAudioTrack failed",
					slog.String("sim", packet.Sim),
					slog.Any("err", err))
			}
		} else {
			w.audioChan <- packet
		}
	default:
		slog.Warn("unknown packet type",
			slog.Any("packet", packet))
	}
}

func (w *whip) addAudioTrack(packet *jt1078.Packet) error {
	if w.pc == nil {
		return fmt.Errorf("cannot add audio track: PeerConnection is nil")
	}

	mimeType := ""
	switch packet.PT {
	case jt1078.PTG711A:
		mimeType = webrtc.MimeTypePCMA
	case jt1078.PTG711U:
		mimeType = webrtc.MimeTypePCMU
	case jt1078.PTAAC:
		mimeType = webrtc.MimeTypeOpus
	default:
		return fmt.Errorf("unknown packet type: %d", packet.PT)
	}

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: mimeType, ClockRate: 8000, Channels: 1}, // G711 通常 8000/1
		"audio", fmt.Sprintf("pion-whip-audio-%s", packet.Sim),
	)
	if err != nil {
		return err
	}

	_, err = w.pc.AddTrack(track)
	if err != nil {
		return err
	}

	w.audioChan = make(chan *jt1078.Packet, 10)
	return nil
}

func (w *whip) pumpPacketsToTrack(track *webrtc.TrackLocalStaticSample, packetCh <-chan *jt1078.Packet) {
	var (
		lastBody []byte
		lastTs   uint64
		first    = true
	)

	for pkt := range packetCh {
		if first {
			lastBody = pkt.Body
			lastTs = pkt.Timestamp
			first = false
			continue
		}

		duration := time.Duration(pkt.Timestamp-lastTs) * time.Millisecond
		if err := track.WriteSample(media.Sample{
			Data:     lastBody,
			Duration: duration,
		}); err != nil {
			slog.Warn("WriteSample failed",
				slog.Any("err", err))
			return
		}

		lastBody = pkt.Body
		lastTs = pkt.Timestamp
	}
}
