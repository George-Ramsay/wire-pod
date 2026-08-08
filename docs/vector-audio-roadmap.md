# Vector desktop audio roadmap

## Goal

Make Vector usable as a normal desktop microphone and speaker so applications such as Codex Voice, ChatGPT, Discord, browsers, and other software can use the robot without Vector-specific integrations.

The audio path should prioritize conversational latency and preserve Vector's existing personality and WirePod functionality.

## Ticket backlog

### T1. Low-latency speaker streaming

**Status:** Prototype implemented on `agent/vector-audio-speaker-stream`; physical-robot latency validation remains.

Expose a streaming endpoint that accepts live signed 16-bit mono PCM and forwards chunks directly to Vector's `ExternalAudioStreamPlayback` RPC instead of buffering an entire file first.

Acceptance criteria:
- Raw PCM can begin playing before the HTTP request finishes.
- 8 kHz and 16 kHz voice audio are supported.
- Chunk duration and volume are configurable.
- Only one external playback stream is active per robot.
- Client disconnects stop the stream promptly.
- Existing `/api-sdk/play_sound` behavior remains unchanged.
- Parameter validation has automated tests.
- On-device testing confirms acceptable conversational latency and no repeated buffer overruns.

### T2. Continuous Vector microphone capture

**Status:** Technical spike required before implementation.

Expose Vector's microphone audio as a continuous PCM stream suitable for a desktop audio bridge.

The current Vector protocol still exposes the `AudioFeed` RPC and defines 1,600-sample SDK audio messages plus 15,625 Hz microphone and 16,000 Hz processed sample-rate constants. Older Vector technical documentation says the public `AudioFeed` command was defined but not implemented on some production firmware, so the first step is to probe the actual robot. If the RPC is unavailable, capture should instead branch from WirePod's existing incoming voice/audio path or use firmware support where appropriate.

Acceptance criteria:
- Confirm whether `AudioFeed` works on the target production Vector and record the actual PCM format/sample rate.
- Capture does not require the user to say the normal Vector wake phrase for every utterance.
- Output format is documented and stable.
- Capture can run for extended conversations.
- Existing WirePod speech recognition can coexist with, or explicitly hand off to, bridge mode.
- Disconnects and robot reconnections recover cleanly.

### T3. Desktop audio bridge and selectable devices

Create the host-side bridge that maps computer audio to Vector and Vector microphone audio back to the computer.

Acceptance criteria:
- The host can expose a `Vector Speaker` output and `Vector Microphone` input.
- Applications can select the devices independently of the system default.
- Windows 10/11 is the first supported desktop target, with the transport layer kept portable for later macOS/Linux support.
- Audio conversion/resampling happens outside Vector when required.
- The bridge reports current robot, connection state, sample rate, buffer depth, and measured transport latency.

### T4. Full-duplex conversation and echo handling

Prevent Vector's speaker output from being fed back as new microphone input while preserving the ability to interrupt speech naturally.

Acceptance criteria:
- Codex/voice assistants do not hear their own responses as user speech.
- User barge-in remains possible while Vector is speaking.
- Start with a simple playback-reference suppressor if necessary, then evaluate acoustic echo cancellation.
- Added processing does not materially increase conversational latency.

### T5. Audio-reactive Vector behavior

Add optional physical reactions driven by bridge state and audio activity without putting animation work on the critical audio path.

Acceptance criteria:
- Listening, speaking, idle, error, and disconnect states can trigger distinct behavior.
- Speaking motion can follow output amplitude at a low update rate.
- Animations are asynchronous and cannot block or delay PCM delivery.
- Feature can be disabled independently of audio streaming.

### T6. Reliability, reconnect, and lifecycle

Make the bridge safe to leave running every day alongside WirePod.

Acceptance criteria:
- Automatically reconnect after robot Wi-Fi loss or WirePod restart.
- Avoid stale SDK connections and overlapping playback sessions.
- Cleanly release audio resources when a client exits.
- Provide useful logs for underruns, overruns, disconnects, and reconnects.
- Preserve normal WirePod startup and existing robot functionality when the bridge is unused.

### T7. Packaging and configuration

Integrate the completed bridge into the normal WirePod installation experience.

Acceptance criteria:
- Bridge settings are configurable without editing source code.
- Existing installations can opt in without losing current WirePod configuration.
- The upper-case `WirePod` packaging repository can point at and package the customized server fork when ready.
- Upgrade/rebase instructions are documented so upstream WirePod updates can still be incorporated.

## Initial playback prototype

The first branch adds:

```text
POST /api-vector-audio/playback?serial=<ESN>&sample_rate=16000&volume=100&chunk_ms=20
Content-Type: application/octet-stream
Body: signed 16-bit little-endian mono PCM, written at approximately real-time speed
```

It also adds `chipper/cmd/vector-audio-send`, a small stdin-to-Vector streaming client. A useful first physical test is to produce real-time 16 kHz mono signed-16 PCM with ffmpeg and pipe it into the client:

```bash
cd chipper
ffmpeg -re -i test.wav -ac 1 -ar 16000 -f s16le - | \
  go run ./cmd/vector-audio-send -serial <VECTOR_ESN>
```

The sender itself uses only the Go standard library and has been locally compiled successfully. The full WirePod package still requires on-repository/on-device validation because this development environment cannot connect to the user's Vector or fetch the complete WirePod build dependencies.

The desktop bridge should keep this HTTP transport internal. End users should eventually interact with normal operating-system audio-device selectors rather than this endpoint.

## Repository note

GitHub Issues were disabled on this fork when this roadmap was created, so these tickets are temporarily tracked here. Once Issues are enabled, each `T#` section should be promoted to its own GitHub issue and linked back to this roadmap.
