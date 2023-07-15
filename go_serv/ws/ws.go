package ws

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

type Room struct {
	Streamers [2]*Streamer
	Viewer    *Viewer
	trackChan chan *webrtc.TrackLocalStaticRTP
}

type Viewer struct {
	conn *websocket.Conn
	pc   *webrtc.PeerConnection
}

type Streamer struct {
	conn *websocket.Conn
	pc   *webrtc.PeerConnection
}

type Message struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

var rooms map[string]*Room
var roomsMutex sync.RWMutex

func HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	if rooms == nil {
		rooms = make(map[string]*Room)
	}

	roomsMutex.Lock()
	_, ok := rooms["test"]
	if !ok {
		rooms["test"] = &Room{
			Streamers: [2]*Streamer{},
			Viewer:    nil,
			trackChan: make(chan *webrtc.TrackLocalStaticRTP),
		}
	}
	roomsMutex.Unlock()

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			return
		}
		switch msg.Type {
		case "offer":
			handleOffer(conn, msg.Data)
		case "viewerOffer":
			handleViewerOffer(conn, msg.Data)
		case "candidate":
			handleCandidate(conn, msg.Data)
		default:
		}
	}
}

func handleOffer(conn *websocket.Conn, data string) {
	roomsMutex.Lock()
	room := rooms["test"]
	roomsMutex.Unlock()

	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	opts := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	pc, err := webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(opts)
	if err != nil {
		log.Println(err)
		return
	}

	// Handle incoming tracks from the remote peer
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		// Print incoming track details
		if track.Kind() == webrtc.RTPCodecTypeVideo {
			localTrack, _ := webrtc.NewTrackLocalStaticRTP(track.Codec().RTPCodecCapability, track.ID(), track.StreamID())

			room.trackChan <- localTrack

			rtpBuf := make([]byte, 1400)
			for {
				i, _, readErr := track.Read(rtpBuf)
				if readErr != nil {
					panic(readErr)
				}

				// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
				if _, err = localTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					panic(err)
				}
			}
		}
	})

	offer := webrtc.SessionDescription{}
	err = json.Unmarshal([]byte(data), &offer)
	if err != nil {
		log.Println(err)
		return
	}

	err = pc.SetRemoteDescription(offer)
	if err != nil {
		log.Println(err)
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Println(err)
		return
	}

	err = pc.SetLocalDescription(answer)
	if err != nil {
		log.Println(err)
		return
	}

	answerMessage := Message{
		Type: "answer",
		Data: pc.CurrentLocalDescription().SDP,
	}

	room.Streamers[0] = &Streamer{conn: conn, pc: pc}

	conn.WriteJSON(answerMessage)
}

func handleViewerOffer(conn *websocket.Conn, data string) {
	roomsMutex.Lock()
	room := rooms["test"]
	roomsMutex.Unlock()

	track := <-room.trackChan

	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	opts := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	pc, err := webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(opts)
	if err != nil {
		log.Println(err)
		return
	}

	// // Handle incoming tracks from the remote peer
	// pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	// 	// Print incoming track details
	// 	if track.Kind() == webrtc.RTPCodecTypeVideo {
	// 		localTrack, _ := webrtc.NewTrackLocalStaticRTP(track.Codec().RTPCodecCapability, track.ID(), track.StreamID())

	// 		room.trackChan <- localTrack

	// 		rtpBuf := make([]byte, 1400)
	// 		for {
	// 			i, _, readErr := track.Read(rtpBuf)
	// 			if readErr != nil {
	// 				panic(readErr)
	// 			}

	// 			// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
	// 			if _, err = localTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
	// 				panic(err)
	// 			}
	// 		}
	// 	}
	// })
	//pc.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{})

	// go func() {
	// 	rtcpBuf := make([]byte, 1500)
	// 	for {
	// 		if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
	// 			return
	// 		}
	// 	}
	// }()
	// if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
	// 	panic(err)
	// }
	rtpSender, err := pc.AddTrack(track)
	if err != nil {
		log.Println(err)
		return
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	offer := webrtc.SessionDescription{}
	err = json.Unmarshal([]byte(data), &offer)
	if err != nil {
		log.Println(err)
		return
	}

	err = pc.SetRemoteDescription(offer)
	if err != nil {
		log.Println(err)
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Println(err)
		return
	}

	err = pc.SetLocalDescription(answer)
	if err != nil {
		log.Println(err)
		return
	}

	answerMessage := Message{
		Type: "answer",
		Data: pc.CurrentLocalDescription().SDP,
	}

	room.Streamers[0] = &Streamer{conn: conn, pc: pc}

	conn.WriteJSON(answerMessage)
}

// func handleViewerOffer(conn *websocket.Conn, data string) {
// 	roomsMutex.Lock()
// 	room := rooms["test"]
// 	roomsMutex.Unlock()

// 	// track := <-room.trackChan

// 	m := &webrtc.MediaEngine{}
// 	if err := m.RegisterDefaultCodecs(); err != nil {
// 		panic(err)
// 	}

// 	opts := webrtc.Configuration{
// 		ICEServers: []webrtc.ICEServer{
// 			{
// 				URLs: []string{"stun:stun.l.google.com:19302"},
// 			},
// 		},
// 	}

// 	pc, err := webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(opts)
// 	if err != nil {
// 		log.Println(err)
// 		return
// 	}

// 	// rtpSender, err := pc.AddTrack(track)
// 	// if err != nil {
// 	// 	log.Println(err)
// 	// 	return
// 	// }

// 	// go func() {
// 	// 	rtcpBuf := make([]byte, 1500)
// 	// 	for {
// 	// 		if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
// 	// 			return
// 	// 		}
// 	// 	}
// 	// }()

// 	offer := webrtc.SessionDescription{}
// 	err = json.Unmarshal([]byte(data), &offer)
// 	if err != nil {
// 		log.Println(err)
// 		return
// 	}

// 	err = pc.SetRemoteDescription(offer)
// 	if err != nil {
// 		log.Println(err)
// 		return
// 	}

// 	answer, err := pc.CreateAnswer(nil)
// 	if err != nil {
// 		log.Println(err)
// 		return
// 	}

// 	err = pc.SetLocalDescription(answer)
// 	if err != nil {
// 		log.Println(err)
// 		return
// 	}

// 	answerMessage := Message{
// 		Type: "answer",
// 		Data: pc.CurrentLocalDescription().SDP,
// 	}

// 	room.Viewer = &Viewer{conn: conn, pc: pc}

// 	conn.WriteJSON(answerMessage)
// }

type Candidate struct {
	Candidate        string `json:"candidate"`
	SDPMid           string `json:"sdpMid"`
	SDPMLineIndex    uint16 `json:"sdpMLineIndex"`
	UsernameFragment string `json:"usernameFragment"`
}

func handleCandidate(conn *websocket.Conn, data string) {
	roomsMutex.Lock()
	room := rooms["test"]
	roomsMutex.Unlock()

	var candidate Candidate
	err := json.Unmarshal([]byte(data), &candidate)
	if err != nil {
		log.Println(err)
		return
	}

	iceCandidate := webrtc.ICECandidateInit{
		Candidate:        candidate.Candidate,
		SDPMid:           &candidate.SDPMid,
		SDPMLineIndex:    &candidate.SDPMLineIndex,
		UsernameFragment: &candidate.UsernameFragment,
	}
	err = room.Streamers[0].pc.AddICECandidate(iceCandidate)
	if err != nil {
		log.Println(err)
		return
	}
}
