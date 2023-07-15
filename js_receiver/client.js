let peer;
const ws = new WebSocket("ws://localhost:8080/ws");
const remoteVideo = document.getElementById("rcvstream");

const setupStream = async () => {
  peer = await createpeer();
  peer.addTransceiver("video");

  const offer = await peer.createOffer();
  await peer.setLocalDescription(offer);

  peer.ontrack = (event) => {
    if (event.track.kind === "video" && !event.track.ended) {
      remoteVideo.srcObject = event.streams[0];
      console.log(remoteVideo.srcObject);
    }
  };

  const message = {
    type: "viewerOffer",
    data: JSON.stringify(peer.localDescription),
  };
  ws.send(JSON.stringify(message));
};

ws.onopen = () => {
  console.log("Connected to signaling server");
  setupStream();
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  switch (data.type) {
    case "answer":
      handleAnswer(data);
      break;
    default:
      break;
  }
};

const createpeer = async () => {
  const pc = new RTCPeerConnection({
    iceServers: [
      {
        urls: "stun:stun.l.google.com:19302",
      },
    ],
  });


  pc.onicegatheringstatechange = () => console.log(pc.iceGatheringState);
  pc.oniceconnectionstatechange = () => console.log(pc.iceConnectionState);
  pc.onicecandidate = (event) => handleIceCandidateEvent(event);
  pc.onconnectionstatechange = (event) => console.log(event);

  return pc;
};

const handleIceCandidateEvent = (event) => {
  if (event.candidate) {
    const message = {
      type: "candidate",
      data: JSON.stringify(event.candidate),
    };
    ws.send(JSON.stringify(message));
  } 
};

const handleAnswer = async (answerPayload) => {
  await peer.setRemoteDescription(
    new RTCSessionDescription({
      type: "answer",
      sdp: answerPayload.data,
    })
  );
};
