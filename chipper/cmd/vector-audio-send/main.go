package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	wirePodURL := flag.String("wirepod", "http://127.0.0.1", "WirePod base URL")
	serial := flag.String("serial", "", "Vector ESN/serial number")
	sampleRate := flag.Int("sample-rate", 16000, "PCM sample rate: 8000 or 16000 Hz")
	volume := flag.Int("volume", 100, "Vector playback volume: 0-100")
	chunkMS := flag.Int("chunk-ms", 20, "server playback chunk size in milliseconds: 10-100")
	flag.Parse()

	if strings.TrimSpace(*serial) == "" {
		fatalf("-serial is required")
	}
	if *sampleRate != 8000 && *sampleRate != 16000 {
		fatalf("-sample-rate must be 8000 or 16000")
	}
	if *volume < 0 || *volume > 100 {
		fatalf("-volume must be between 0 and 100")
	}
	if *chunkMS < 10 || *chunkMS > 100 {
		fatalf("-chunk-ms must be between 10 and 100")
	}

	endpoint, err := url.Parse(strings.TrimRight(*wirePodURL, "/") + "/api-vector-audio/playback")
	if err != nil {
		fatalf("invalid WirePod URL: %v", err)
	}
	query := endpoint.Query()
	query.Set("serial", *serial)
	query.Set("sample_rate", strconv.Itoa(*sampleRate))
	query.Set("volume", strconv.Itoa(*volume))
	query.Set("chunk_ms", strconv.Itoa(*chunkMS))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodPost, endpoint.String(), os.Stdin)
	if err != nil {
		fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	// No overall request timeout: a microphone/output capture stream may remain
	// open for hours. Transport timeouts still detect a dead WirePod connection.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:           (&netDialer).DialContext,
			ForceAttemptHTTP2:     false,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}

	fmt.Fprintf(os.Stderr, "Streaming stdin to Vector %s via %s\n", *serial, endpoint.String())
	resp, err := client.Do(req)
	if err != nil {
		fatalf("stream audio: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fatalf("WirePod returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	fmt.Fprintln(os.Stderr, "Vector audio stream completed")
}

var netDialer = net.Dialer{
	Timeout:   5 * time.Second,
	KeepAlive: 30 * time.Second,
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "vector-audio-send: "+format+"\n", args...)
	os.Exit(1)
}
