package iowrappers

import (
	"errors"
	"testing"
	"time"
)

// TestCanServeFromCache covers the defect behind the repeated Google fan-outs: a cached result of
// zero could never be suppressed by the freshness marker, so every request for a sparse
// (category, area) re-ran the full external search indefinitely.
func TestCanServeFromCache(t *testing.T) {
	readFailure := errors.New("redis read failed")
	markerAbsent := errors.New("redis: nil")

	cases := []struct {
		name        string
		cachedCount int
		readErr     error
		markerMiss  error
		markerAge   time.Duration
		want        bool
	}{
		{
			name:        "empty result with a recent marker is served from cache",
			cachedCount: 0,
			markerAge:   time.Hour,
			want:        true,
		},
		{
			name:        "empty result past the empty-result window triggers a search",
			cachedCount: 0,
			markerAge:   MinEmptyResultRefreshDuration + time.Hour,
			want:        false,
		},
		{
			name:        "empty result exactly at the window boundary is still served",
			cachedCount: 0,
			markerAge:   MinEmptyResultRefreshDuration,
			want:        true,
		},
		{
			// An empty result must NOT get the full populated window, or a genuinely new area
			// stays empty for two weeks.
			name:        "empty result inside the populated window but past the empty one searches",
			cachedCount: 0,
			markerAge:   MinMapsResultRefreshDuration - time.Hour,
			want:        false,
		},
		{
			name:        "populated result within the refresh window is served from cache",
			cachedCount: 12,
			markerAge:   MinMapsResultRefreshDuration - time.Hour,
			want:        true,
		},
		{
			name:        "populated result past the refresh window triggers a search",
			cachedCount: 12,
			markerAge:   MinMapsResultRefreshDuration + time.Hour,
			want:        false,
		},
		{
			name:        "a missing marker always triggers a search",
			cachedCount: 12,
			markerMiss:  markerAbsent,
			markerAge:   time.Hour,
			want:        false,
		},
		{
			name:        "a failed read always triggers a search",
			cachedCount: 12,
			readErr:     readFailure,
			markerAge:   time.Hour,
			want:        false,
		},
		{
			// Both signals bad: still one search, never a served-from-cache result.
			name:        "a failed read with a missing marker triggers a search",
			cachedCount: 0,
			readErr:     readFailure,
			markerMiss:  markerAbsent,
			markerAge:   time.Hour,
			want:        false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canServeFromCache(tc.cachedCount, tc.readErr, tc.markerMiss, tc.markerAge)
			if got != tc.want {
				t.Errorf("canServeFromCache(%d, %v, %v, %v) = %v, want %v",
					tc.cachedCount, tc.readErr, tc.markerMiss, tc.markerAge, got, tc.want)
			}
		})
	}
}

// TestEmptyRefreshWindowIsShorter guards the relationship the two constants must keep. If the
// empty window ever reached the populated one, an area that returned nothing would be frozen for
// the full period; if it were zero, empty results would fan out on every request again.
func TestEmptyRefreshWindowIsShorter(t *testing.T) {
	if MinEmptyResultRefreshDuration <= 0 {
		t.Errorf("MinEmptyResultRefreshDuration = %v, must be positive or empty results re-search every request", MinEmptyResultRefreshDuration)
	}
	if MinEmptyResultRefreshDuration >= MinMapsResultRefreshDuration {
		t.Errorf("MinEmptyResultRefreshDuration (%v) must be shorter than MinMapsResultRefreshDuration (%v)",
			MinEmptyResultRefreshDuration, MinMapsResultRefreshDuration)
	}
}
