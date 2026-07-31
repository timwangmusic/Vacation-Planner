package redis_client_mocks

import (
	"errors"
	"testing"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

// placeSearchCandidateFixtureIDs lists every place ID this file writes, so cleanup can remove
// exactly these keys rather than touching the shared RedisClient/RedisMockSvr fixtures other test
// files in this package depend on (see bucket_cleanup_test.go's resetBucketCleanupFixtures for the
// established pattern). NEVER FlushAll here.
var placeSearchCandidateFixtureIDs = []string{
	"psc-round-trip-1", "psc-ttl-1", "psc-expire-1", "psc-collision-1",
}

func cleanupPlaceSearchCandidateFixtures(t *testing.T) {
	t.Helper()
	keys := make([]string, 0, len(placeSearchCandidateFixtureIDs)*2)
	for _, id := range placeSearchCandidateFixtureIDs {
		keys = append(keys,
			iowrappers.PlaceSearchCandidateRedisKeyPrefix+id,
			iowrappers.PlaceDetailsRedisKeyPrefix+id,
		)
	}
	if err := RedisClient.RemoveKeys(RedisContext, keys); err != nil {
		t.Fatalf("RemoveKeys: %v", err)
	}
}

func TestPlaceSearchCandidate_RoundTripPreservesTypes(t *testing.T) {
	cleanupPlaceSearchCandidateFixtures(t)
	t.Cleanup(func() { cleanupPlaceSearchCandidateFixtures(t) })

	place := POI.Place{
		ID:           "psc-round-trip-1",
		Name:         "Test Museum",
		LocationType: POI.LocationTypeMuseum,
		Types:        []string{"museum", "point_of_interest", "establishment"},
		Location:     POI.Location{Latitude: 1.23, Longitude: 4.56},
	}

	if err := RedisClient.SetPlaceSearchCandidate(RedisContext, place, iowrappers.PlaceSearchCandidateTTL); err != nil {
		t.Fatalf("SetPlaceSearchCandidate: %v", err)
	}

	got, err := RedisClient.PlaceSearchCandidate(RedisContext, place.ID)
	if err != nil {
		t.Fatalf("PlaceSearchCandidate: %v", err)
	}
	if got.ID != place.ID || got.Name != place.Name {
		t.Errorf("round trip mismatch: got %+v, want ID/Name from %+v", got, place)
	}
	if len(got.Types) != len(place.Types) {
		t.Fatalf("Types not preserved: got %+v, want %+v", got.Types, place.Types)
	}
	for i := range place.Types {
		if got.Types[i] != place.Types[i] {
			t.Errorf("Types[%d] = %q, want %q", i, got.Types[i], place.Types[i])
		}
	}
}

func TestPlaceSearchCandidate_TTLIsSet(t *testing.T) {
	cleanupPlaceSearchCandidateFixtures(t)
	t.Cleanup(func() { cleanupPlaceSearchCandidateFixtures(t) })

	place := POI.Place{ID: "psc-ttl-1", Name: "TTL Test"}
	if err := RedisClient.SetPlaceSearchCandidate(RedisContext, place, iowrappers.PlaceSearchCandidateTTL); err != nil {
		t.Fatalf("SetPlaceSearchCandidate: %v", err)
	}

	ttl := RedisClient.Get().TTL(RedisContext, iowrappers.PlaceSearchCandidateRedisKeyPrefix+place.ID).Val()
	if ttl <= 0 || ttl > iowrappers.PlaceSearchCandidateTTL {
		t.Errorf("TTL = %v, want in (0, %v]", ttl, iowrappers.PlaceSearchCandidateTTL)
	}
}

func TestPlaceSearchCandidate_ExpiresAfterTTL(t *testing.T) {
	cleanupPlaceSearchCandidateFixtures(t)
	t.Cleanup(func() { cleanupPlaceSearchCandidateFixtures(t) })

	place := POI.Place{ID: "psc-expire-1", Name: "Expiring Candidate"}
	if err := RedisClient.SetPlaceSearchCandidate(RedisContext, place, iowrappers.PlaceSearchCandidateTTL); err != nil {
		t.Fatalf("SetPlaceSearchCandidate: %v", err)
	}

	// Advance the mock server's virtual clock past the 30-minute TTL. This is scoped to keys this
	// test owns; no other fixture in this package sets a TTL under 31 minutes (checked at the time
	// this test was written), so this does not prematurely expire unrelated tests' data.
	RedisMockSvr.FastForward(31 * time.Minute)

	_, err := RedisClient.PlaceSearchCandidate(RedisContext, place.ID)
	if !errors.Is(err, iowrappers.ErrSearchCandidateNotFound) {
		t.Errorf("PlaceSearchCandidate after expiry: got err %v, want it to wrap ErrSearchCandidateNotFound", err)
	}
}

func TestPlaceSearchCandidate_MissReturnsNotFoundSentinel(t *testing.T) {
	_, err := RedisClient.PlaceSearchCandidate(RedisContext, "psc-never-searched-at-all")
	if !errors.Is(err, iowrappers.ErrSearchCandidateNotFound) {
		t.Errorf("got err %v, want it to wrap ErrSearchCandidateNotFound", err)
	}
}

// TestPlaceSearchCandidate_PrefixDoesNotCollideWithPlaceDetails pins that the candidate stash key
// space (place_search_candidate:place_ID:<id>) is distinct from the permanent place record key
// space (place_details:place_ID:<id>) for the same place ID: writing one must not be readable
// through the other.
func TestPlaceSearchCandidate_PrefixDoesNotCollideWithPlaceDetails(t *testing.T) {
	cleanupPlaceSearchCandidateFixtures(t)
	t.Cleanup(func() { cleanupPlaceSearchCandidateFixtures(t) })

	id := "psc-collision-1"
	candidate := POI.Place{ID: id, Name: "Collision Candidate"}
	details := POI.Place{ID: id, Name: "Collision Details", LocationType: POI.LocationTypeMuseum}

	if err := RedisClient.SetPlaceSearchCandidate(RedisContext, candidate, iowrappers.PlaceSearchCandidateTTL); err != nil {
		t.Fatalf("SetPlaceSearchCandidate: %v", err)
	}
	if err := RedisClient.SetPlace(RedisContext, details); err != nil {
		t.Fatalf("SetPlace: %v", err)
	}

	gotCandidate, err := RedisClient.PlaceSearchCandidate(RedisContext, id)
	if err != nil {
		t.Fatalf("PlaceSearchCandidate: %v", err)
	}
	if gotCandidate.Name != "Collision Candidate" {
		t.Errorf("PlaceSearchCandidate returned %q, want %q (key prefix collision with place_details?)",
			gotCandidate.Name, "Collision Candidate")
	}

	gotDetails, err := RedisClient.CachedPlaces(RedisContext, []string{id})
	if err != nil {
		t.Fatalf("CachedPlaces: %v", err)
	}
	if gotDetails[id].Name != "Collision Details" {
		t.Errorf("CachedPlaces returned %q, want %q (key prefix collision with place_search_candidate?)",
			gotDetails[id].Name, "Collision Details")
	}
}
