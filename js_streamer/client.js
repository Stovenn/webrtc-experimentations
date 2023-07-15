let peer;
const ws = new WebSocket("ws://localhost:8080/ws");
const localVideo = document.getElementById("sendstream");

const setupStream = async () => {
  peer = await createpeer();

  const stream = await navigator.mediaDevices.getUserMedia({
    video: true,
    audio: true,
  });
  console.log(stream)
  localVideo.srcObject = stream;

  stream.getTracks().forEach((track) => {
    peer.addTrack(track, stream);
  });

  const offer = await peer.createOffer();
  await peer.setLocalDescription(offer);

  const message = {
    type: "offer",
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

const handleNegotiationNeededEvent = async () => {
  const offer = await peer.createOffer();
  await peer.setLocalDescription(offer);

  const message = {
    type: "offer",
    data: JSON.stringify(peer.localDescription),
  };
  ws.send(JSON.stringify(message));
};

const handleAnswer = async (answerPayload) => {
  await peer.setRemoteDescription(
    new RTCSessionDescription({
      type: "answer",
      sdp: answerPayload.data,
    })
  );
};




