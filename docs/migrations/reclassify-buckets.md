# Migration: purge misclassified places from category geo buckets

One-time cleanup for the `fast_food_restaurant` incident (#446). The legacy Nearby
Search ignores an unknown `?type=` instead of erroring, so searches for two
Places-API-(New)-only types returned prominence-ranked establishments that were then
stamped with the queried type — writing hotels into the eatery geo bucket.

> **Note:** this document predates the bucket collapse. Eateries were split across
> `placeIDs:eatery:level0..4` when the incident happened; they are now a single
> `placeIDs:eatery` bucket. See [collapse-eatery-buckets.md](collapse-eatery-buckets.md).
> The migration itself is unaffected — it resolves keys through
> `POI.EncodeNearbySearchRedisKey` — but the member counts below were measured pre-collapse.

`GET /v1/migrate/reclassify-buckets?category=Eatery` — admin only, **dry-run unless
`apply=true`**. Any other value of `apply` (absent, empty, `TRUE`, `1`) is a dry run.

## Run the fixes first

This must run **after** the write-path fixes in this PR are deployed. `8644199` is still
creating new bad records in production: every city/category/price-bucket whose
`MapsLastSearchTime` marker passes the 14-day `MinMapsResultRefreshDuration` does a fresh
cold search. Cleaning before the fix deploys just gets re-polluted.

## The rule

A member is removed only when its **primary** Google type maps to a *different*
category — the exact inverse of the write rule. An unmapped primary type is not
evidence of misclassification, because the write path would legitimately place it there.

| Primary type | Resolves to | Action |
| --- | --- | --- |
| `lodging` | Lodging | removed |
| `supermarket`, `department_store` | Shopping | removed |
| `cafe`, `restaurant` | Eatery | kept |
| `meal_delivery`, `night_club` | Eatery (positively mapped) | kept |
| `stadium` (found in the Eatery bucket, per the production audit below) | Visit | removed |
| `hardware_store`, `shoe_store`, `furniture_store` (found in the Eatery bucket, per the production audit below) | Shopping | removed |
| `movie_theater` (rule example only — pinned by the test truth table; never observed in the production Eatery bucket, see Known residue) | Visit | removed |
| `university`, `airport`, `real_estate_agency`, `doctor`, `finance`, `local_government_office`, `general_contractor`, `veterinary_care` | unmapped | kept |
| no `types[]` at all | unmapped | kept |

`meal_delivery` and `night_club` used to fall in the "unmapped, kept" row above: the map had
no entry for them, so `GetPlaceCategory` returned `ok=false` and the migration kept them for
lack of any evidence of misclassification. The place-type map expansion (`POI/categories.go`,
`placeTypeToCategory`) added them as positive Eatery entries, so a `night_club`-primaried
member of the Eatery bucket is no longer residue by omission — it is now recognized as
correctly filed. The verdict (kept) is unchanged; the reason changed from "unmapped, benefit
of the doubt" to "positively belongs here." Conversely, `stadium`, `hardware_store`,
`shoe_store`, and `furniture_store` used to be unmapped-and-kept too; they are now positively
mapped to a category *other than* Eatery (Visit for `stadium`; Shopping for the other three —
`shoe_store` and `furniture_store` are two more instances of the `*_store` → Shopping
expansion that also added `hardware_store`), so a bucket member primaried with one of them is
newly REMOVABLE when this migration runs against `category=Eatery`. `movie_theater` follows
the identical rule (it maps to Visit) but, unlike the four types above, was never actually
found in the production Eatery bucket — it appears only as a synthetic fixture in
`TestRemoveMisclassifiedPlacesPrimaryTypeTruthTable`, included there to pin the rule generally
rather than to model an observed record.
`TestRemoveMisclassifiedPlacesPrimaryTypeTruthTable` (`test/redis_client_mocks/bucket_cleanup_test.go`)
pins all of these verdicts, including the newly-removable types.

⚠️ Two different pairs of rules are in play here, and only one of them is now unified:

- **Unified:** this migration's removal rule and the read filter the merchant endpoint applies
  (`POI.ReclassifyForCategory`) both now key off the place's **primary** Google type
  (`POI.PrimaryLocationType(place.Types)`) through the same `placeTypeToCategory` table via
  `POI.GetPlaceCategory`. That unification landed in the place-text-search PR ("Expand
  place-type reverse map and unify ReclassifyForCategory on it") and is intentional — one table
  now decides "does this place belong in this category" for both the admin cleanup path and the
  merchant-endpoint read path. (The two are not byte-for-byte identical in every case — the read
  filter drops a place whose primary type is present but still unmapped, while this migration's
  removal rule treats that same case as "not evidence of misclassification" and leaves it
  in the bucket — but they agree on the case that matters for safety: a primary type that
  positively resolves to a *different* category is excluded by both.)
- **Must stay apart:** this migration's removal rule and the **write** rule
  (`SetPlacesAddGeoLocations`, which keys on the place's stamped `LocationType` — the type it was
  *searched* under, not necessarily its true primary type) deliberately disagree, and must keep
  disagreeing. A refactor that "unifies" the removal rule with the write rule reintroduces the
  `fast_food_restaurant` incident: the write rule's whole job is to accept whatever type a search
  was run under, and folding the primary-type check into it would let an unenforceable `?type=`
  silently relabel results again.

`TestRemoveMisclassifiedPlacesPrimaryTypeTruthTable` pins the dangerous direction.

## Expected scale

Measured against production on 2026-07-30 (24,203 bucket members scanned):

| Category | Members | Candidates |
| --- | --- | --- |
| Eatery | 8,589 | 145 |
| Visit | 14,674 | 0 |
| Shopping | 498 | 12 |
| Wellness | 319 | 4 |
| Lodging | 123 | 3 |

**164 total.** Of these, 123 are incident-attributable (stamped `fast_food_restaurant`,
mostly San Francisco / Tulsa / Boise hotels); the other 41 are pre-existing
misclassifications the rule also catches (hotels stamped `restaurant`, supermarkets
stamped `bakery`).

⚠️ The numbers above (145/0/12/4/3, 164 total) predate the place-type map expansion. The next
dry run must be diffed against *these* baseline numbers, and the counts are **expected to
change** — the removal rule now recognizes primary types (`stadium`, `hardware_store`,
`shoe_store`, `furniture_store`, and others) it used to treat as unmapped-and-kept, so the
Eatery candidate count in particular should go up by (at least) the 5 records itemized below.
A jump versus this baseline is not by itself a red flag; it is what widening the map is
supposed to do. What it does mean: **do not apply blind**. Spot-check each newly-appearing
candidate class (i.e. each distinct primary type showing up in `RemovedIDs` that wasn't there
before) against a few real records before running with `apply=true`, the same way
`123 fast_food_restaurant` vs `41 pre-existing` was broken out above.

**Known residue:** complete production audit (2026-07-30) of all 27 records in `placeIDs:eatery`
whose primary type mapped to no category under the pre-expansion map:

| Primary type | Count | Example |
| --- | ---: | --- |
| (no `types[]`) | 7 | Williams Co Inc, The Tulsa Theater, Oktoberfest Main Office, Sun Valley Music Festival |
| `university` | 5 | Ohlone College, The University of Tulsa, Boise State University, Concordia College |
| `shoe_store` | 2 | LOFT, JoS. A. Bank |
| `airport` | 2 | Boise Airport, Sun Valley Gun Club |
| `real_estate_agency` | 2 | Avalon Mountain View, Mission Peaks Apartments |
| `furniture_store` | 1 | Topnotch Fine Furnishings & Interior Design |
| `hardware_store` | 1 | The Home Depot |
| `finance` | 1 | The UPS Store |
| `doctor` | 1 | Ricardo Delgado, MD |
| `local_government_office` | 1 | Tulsa County Assessor |
| `stadium` | 1 | BOK Center |
| `night_club` | 1 | Cain's Ballroom |
| `general_contractor` | 1 | Pella Windows and Doors Showroom of Ketchum, ID |
| `veterinary_care` | 1 | Sun Valley Animal Center |

(An earlier version of this table elided 7 of these records — `shoe_store` ×2,
`furniture_store`, `finance`, `local_government_office`, `general_contractor`, and
`veterinary_care` — which is why its stated "27" total didn't match its own itemization. The
table above is the complete audit.)

With the expanded `placeTypeToCategory` map, this splits three ways:

- **Removable (5):** `shoe_store` ×2 and `furniture_store` ×1 are now positively mapped to
  Shopping, the same `*_store` → Shopping expansion that also added `hardware_store` ×1
  (a strict specialization of the already-mapped `store` type); `stadium` ×1 is now positively
  mapped to Visit. These 4 primary types — 5 records, counting both `shoe_store` instances —
  become removable candidates the next time this migration runs against `category=Eatery`.
- **Legitimized (1):** `night_club` ×1 (Cain's Ballroom) is now positively mapped to Eatery —
  the same category its bucket already files it under — so it is no longer unresolved residue
  at all, just a correctly-classified place.
- **Still residue (21):** `university` ×5, `airport` ×2, `real_estate_agency` ×2, `doctor` ×1,
  `finance` ×1, `local_government_office` ×1, `general_contractor` ×1, `veterinary_care` ×1,
  and the 7 records with no `types[]` at all remain unmapped and kept, for lack of any evidence
  of misclassification — none of those types were added to the map.

**27 → 5 removable + 1 legitimized + 21 residue.** The 21 still-residue records stay in the
eatery buckets. The trip-planning path (`planner/solver.go`) does not reclassify, so they
remain reachable in generated plans. Broadening `GetPlaceCategory` to cover `university`,
`airport`, `real_estate_agency`, `doctor`, `finance`, `local_government_office`,
`general_contractor`, and `veterinary_care` is tracked as follow-up work. This migration has
not been re-run against production since the map expanded; the split above is derived from the
audited pre-expansion snapshot, not from a fresh dry run — re-confirm with one before relying on
it operationally.

Separately, ~328 bucket members have no backing `place_details` record. The migration skips
them. Their likely source has since been fixed: `removePlace` deleted the record but ZREMmed
`placeIDs:eatery:<priceLevel>` — missing the `level` prefix the write path used — and never
touched the Shopping/Lodging/Wellness buckets at all, so it orphaned every member it meant to
remove. `RemovePlaces` now clears every category bucket through the shared encoder, so no new
orphans accumulate; the existing ones still need one `GET /v1/migrate/remove-places` pass.

## Steps

```bash
# 1. Dry run and READ THE OUTPUT before going further.
curl -s -H "Authorization: Bearer $ADMIN_JWT" \
  "https://best-vacation-planner.herokuapp.com/v1/migrate/reclassify-buckets?category=Eatery" | jq

# 2. Apply.
curl -s -H "Authorization: Bearer $ADMIN_JWT" \
  "https://best-vacation-planner.herokuapp.com/v1/migrate/reclassify-buckets?category=Eatery&apply=true" | jq

# 3. Re-run the dry run; misclassified should now be 0 (modulo the residue above).
```

Repeat per category as needed (`Shopping`, `Wellness`, `Lodging`). Reads are pipelined in
batches of 100 and the report carries `bucket_sizes`/`total_members`; the full Eatery scan
completed in ~1.3s against production, well inside Heroku's 30s H12.

To force a cold search when spot-checking an area afterwards, drop its marker field. The format
is now `<cell>:<category>`, where `<cell>` is the search coordinates quantized by
`POI.EncodeSearchCell` — `floor(lat/0.072)_floor(lng/0.072)`. Eatery price levels 3 and 4 add a
`:pricey<N>` segment; nothing else carries a price segment.

```bash
# Los Altos (37.3852, -122.1141) -> floor(37.3852/0.072)=519, floor(-122.1141/0.072)=-1697
redis-cli HDEL MapsLastSearchTime "519_-1697:eatery"

# or find the field for an area you have a request log line for — the debug log emits "cell"
redis-cli HKEYS MapsLastSearchTime | grep ':eatery'
```

⚠️ Marker fields written before the cell change used
`<country>:<admin area 1>:<city>:<category>:<price level>` and are no longer read by anything.
`redis-cli DEL MapsLastSearchTime` is safe cleanup once the new code is deployed — it only
forces one cold search per occupied cell per category, and the geo buckets are untouched.
