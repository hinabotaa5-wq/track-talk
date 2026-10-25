import type { Room, RemoteTrack } from "livekit-client";

export type MeetVoiceCredentials = {
  url: string;
  token: string;
};

// Call only from an explicit join action, with credentials from the Go API.
// Importing this module does not load the SDK, connect, or request microphone access.
export async function connectMeetVoice(
  credentials: MeetVoiceCredentials,
  audioContainer: HTMLElement,
  signal: AbortSignal,
): Promise<{ room: Room; disconnect: () => Promise<void> }> {
  signal.throwIfAborted();
  const { Room, RoomEvent, Track } = await import("livekit-client");
  signal.throwIfAborted();
  const room = new Room({
    disconnectOnPageLeave: true,
    audioCaptureDefaults: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
    publishDefaults: { stopMicTrackOnMute: true },
  });
  const attached = new Map<RemoteTrack, HTMLMediaElement>();
  const detach = (track: RemoteTrack) => {
    const element = attached.get(track);
    if (!element) return;
    track.detach(element);
    element.remove();
    attached.delete(track);
  };
  const attach = (track: RemoteTrack) => {
    if (track.kind !== Track.Kind.Audio || attached.has(track)) return;
    const element = track.attach();
    attached.set(track, element);
    audioContainer.appendChild(element);
  };
  const cleanup = () => {
    for (const track of attached.keys()) detach(track);
    signal.removeEventListener("abort", abort);
    room.off(RoomEvent.TrackSubscribed, attach);
    room.off(RoomEvent.TrackUnsubscribed, detach);
    room.off(RoomEvent.Disconnected, cleanup);
  };
  const disconnect = async () => {
    cleanup();
    await room.disconnect(true);
  };
  const abort = () => { void disconnect().catch(() => {}); };
  room.on(RoomEvent.TrackSubscribed, attach);
  room.on(RoomEvent.TrackUnsubscribed, detach);
  room.on(RoomEvent.Disconnected, cleanup);
  signal.addEventListener("abort", abort, { once: true });
  try {
    // No local track is created or published here. The UI must call
    // setMicrophoneEnabled(true) only after the user's microphone action.
    await room.connect(credentials.url, credentials.token, { autoSubscribe: true });
    signal.throwIfAborted();
    return { room, disconnect };
  } catch (error) {
    await disconnect().catch(() => {});
    throw error;
  }
}
