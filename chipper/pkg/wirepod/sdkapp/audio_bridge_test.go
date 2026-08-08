package sdkapp

import "testing"

func TestParseVectorAudioSampleRate(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "default", raw: "", want: defaultVectorAudioSampleRate},
		{name: "8k", raw: "8000", want: 8000},
		{name: "16k", raw: "16000", want: 16000},
		{name: "unsupported", raw: "44100", wantErr: true},
		{name: "invalid", raw: "voice", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVectorAudioSampleRate(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got sample rate %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseBoundedInt(t *testing.T) {
	got, err := parseBoundedInt("", 20, 10, 100, "chunk_ms")
	if err != nil || got != 20 {
		t.Fatalf("default: got %d, err %v", got, err)
	}

	got, err = parseBoundedInt("10", 20, 10, 100, "chunk_ms")
	if err != nil || got != 10 {
		t.Fatalf("lower bound: got %d, err %v", got, err)
	}

	got, err = parseBoundedInt("100", 20, 10, 100, "chunk_ms")
	if err != nil || got != 100 {
		t.Fatalf("upper bound: got %d, err %v", got, err)
	}

	for _, raw := range []string{"9", "101", "nope"} {
		if _, err := parseBoundedInt(raw, 20, 10, 100, "chunk_ms"); err == nil {
			t.Fatalf("expected %q to fail validation", raw)
		}
	}
}
