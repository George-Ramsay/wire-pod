package sdkapp

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
)

const (
	defaultVectorAudioSampleRate = 16000
	defaultVectorAudioVolume     = 100
	defaultVectorAudioChunkMS    = 20
)

var vectorAudioPlaybackLocks sync.Map

func init() {
	// Keep the streaming endpoint separate from /api-sdk/ so callers can send a
	// raw, chunked PCM request body without r.FormValue parsing it first.
	http.HandleFunc("/api-vector-audio/playback", vectorAudioPlaybackHandler)
}

// vectorAudioPlaybackHandler streams signed 16-bit little-endian mono PCM from
// the HTTP request body directly to Vector's ExternalAudioStreamPlayback RPC.
// The caller is expected to write PCM at approximately real-time speed. This is
// intentional: desktop capture APIs already produce audio in real time, and
// avoiding an extra server-side queue keeps conversational latency low.
//
// Example:
// POST /api-vector-audio/playback?serial=00e20100&sample_rate=16000&volume=100&chunk_ms=20
// Content-Type: application/octet-stream
func vectorAudioPlaybackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serial := strings.TrimSpace(r.URL.Query().Get("serial"))
	if serial == "" {
		http.Error(w, "serial is required", http.StatusBadRequest)
		return
	}

	sampleRate, err := parseVectorAudioSampleRate(r.URL.Query().Get("sample_rate"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	volume, err := parseBoundedInt(r.URL.Query().Get("volume"), defaultVectorAudioVolume, 0, 100, "volume")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chunkMS, err := parseBoundedInt(r.URL.Query().Get("chunk_ms"), defaultVectorAudioChunkMS, 10, 100, "chunk_ms")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	robotObj, robotIndex, err := getRobot(serial)
	if err != nil {
		http.Error(w, "unable to connect to Vector: "+err.Error(), http.StatusBadGateway)
		return
	}
	robots[robotIndex].ConnTimer = 0

	// Vector should only have one external playback stream at a time. Serialize
	// streams per robot instead of allowing two desktop clients to interleave PCM.
	lockValue, _ := vectorAudioPlaybackLocks.LoadOrStore(strings.ToLower(serial), &sync.Mutex{})
	playbackLock := lockValue.(*sync.Mutex)
	playbackLock.Lock()
	defer playbackLock.Unlock()

	audioClient, err := robotObj.Vector.Conn.ExternalAudioStreamPlayback(r.Context())
	if err != nil {
		http.Error(w, "unable to open Vector audio stream: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer audioClient.CloseSend()

	if err := audioClient.SendMsg(&vectorpb.ExternalAudioStreamRequest{
		AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamPrepare{
			AudioStreamPrepare: &vectorpb.ExternalAudioStreamPrepare{
				AudioFrameRate: uint32(sampleRate),
				AudioVolume:    uint32(volume),
			},
		},
	}); err != nil {
		http.Error(w, "unable to prepare Vector audio stream: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Mono signed 16-bit PCM is two bytes per sample. A 20 ms chunk is 640 bytes
	// at 16 kHz, which keeps the bridge responsive while avoiding tiny gRPC writes.
	chunkBytes := sampleRate * 2 * chunkMS / 1000
	if chunkBytes < 2 {
		chunkBytes = 2
	}
	buf := make([]byte, chunkBytes)
	bytesSent := 0

	for {
		n, readErr := io.ReadFull(r.Body, buf)
		if n > 0 {
			chunk := buf[:n]
			if err := audioClient.SendMsg(&vectorpb.ExternalAudioStreamRequest{
				AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamChunk{
					AudioStreamChunk: &vectorpb.ExternalAudioStreamChunk{
						AudioChunkSizeBytes: uint32(len(chunk)),
						AudioChunkSamples:   chunk,
					},
				},
			}); err != nil {
				http.Error(w, "unable to send Vector audio: "+err.Error(), http.StatusBadGateway)
				return
			}
			bytesSent += n
			robots[robotIndex].ConnTimer = 0
		}

		switch readErr {
		case nil:
			continue
		case io.EOF, io.ErrUnexpectedEOF:
			goto complete
		default:
			if r.Context().Err() != nil {
				logger.Println("Vector audio playback client disconnected for " + serial)
				return
			}
			http.Error(w, "unable to read PCM stream: "+readErr.Error(), http.StatusBadRequest)
			return
		}
	}

complete:
	if err := audioClient.SendMsg(&vectorpb.ExternalAudioStreamRequest{
		AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamComplete{
			AudioStreamComplete: &vectorpb.ExternalAudioStreamComplete{},
		},
	}); err != nil {
		http.Error(w, "unable to complete Vector audio stream: "+err.Error(), http.StatusBadGateway)
		return
	}

	logger.Println(fmt.Sprintf("Vector audio playback complete for %s (%d PCM bytes)", serial, bytesSent))
	w.WriteHeader(http.StatusNoContent)
}

func parseVectorAudioSampleRate(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultVectorAudioSampleRate, nil
	}

	sampleRate, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("sample_rate must be 8000 or 16000")
	}
	if sampleRate != 8000 && sampleRate != 16000 {
		return 0, fmt.Errorf("sample_rate must be 8000 or 16000")
	}
	return sampleRate, nil
}

func parseBoundedInt(raw string, defaultValue, min, max int, name string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return value, nil
}
