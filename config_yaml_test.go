package main

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// Pins the Place Details field mask in config/config.yml. Every field here is billed per Details
// call, tiered by field group (Basic / Contact / Atmosphere on the legacy API), so an unused
// field is pure spend:
//   - name and user_ratings_total must stay OUT: neither is read from a Details response on any
//     path (both already arrive free with every Nearby/Text Search result), and
//     user_ratings_total alone pulls the call into the Atmosphere tier. The AddUserRatingsTotal
//     admin migration passes its own single-field list and is unaffected.
//   - the remaining fields are load-bearing: opening_hours (open-now filtering),
//     formatted_address/adr_address (display + address parsing), url (also the Details-freshness
//     signal — see iowrappers/data_migrations.go detailsSourcedFields), editorial_summary (trip
//     planner), photos (place photos + confirm gap-fill).
func TestDetailedSearchFieldsMask(t *testing.T) {
	raw, err := os.ReadFile("config/config.yml")
	if err != nil {
		t.Fatalf("reading config/config.yml: %v", err)
	}
	var configs Configurations
	if err := yaml.Unmarshal(raw, &configs); err != nil {
		t.Fatalf("unmarshal config.yml: %v", err)
	}

	fields := make(map[string]bool)
	for _, f := range configs.Server.GoogleMaps.DetailedSearchFields {
		fields[f] = true
	}

	for _, banned := range []string{"name", "user_ratings_total"} {
		if fields[banned] {
			t.Errorf("detailed_search_fields contains %q, which no Details consumer reads — it only adds billing tier", banned)
		}
	}
	for _, required := range []string{"opening_hours", "formatted_address", "adr_address", "url", "editorial_summary", "photos"} {
		if !fields[required] {
			t.Errorf("detailed_search_fields is missing load-bearing field %q", required)
		}
	}
}
