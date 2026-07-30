# Migration: purge misclassified places from category geo buckets

One-time cleanup for the `fast_food_restaurant` incident (#446). The legacy Nearby
Search ignores an unknown `?type=` instead of erroring, so searches for two
Places-API-(New)-only types returned prominence-ranked establishments that were then
stamped with the queried type — writing hotels into `placeIDs:eatery:level*`.

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
| `meal_delivery`, `night_club` | unmapped | kept |
| no `types[]` at all | unmapped | kept |

⚠️ This rule and the write rule (`SetPlacesAddGeoLocations`, which keys on the stamped
`LocationType`) deliberately disagree. A refactor that "unifies" them reintroduces the
incident. `TestRemoveMisclassifiedPlacesPrimaryTypeTruthTable` pins the dangerous direction.

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

**Known residue:** 27 incident records are *not* removed because their primary type maps
to no category — `university` ×5, `airport` ×2, `real_estate_agency` ×2, `stadium`,
`night_club`, `hardware_store`, `doctor`, and 7 with no `types[]`. These stay in the
eatery buckets. The trip-planning path (`planner/solver.go`) does not reclassify, so they
remain reachable in generated plans. Broadening `GetPlaceCategory` to cover those types is
tracked as follow-up work.

Separately, ~328 bucket members have no backing `place_details` record. Pre-existing and
unrelated; the migration skips them.

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

To force a cold search when spot-checking a city afterwards, drop its marker field
(format `<country>:<admin area 1>:<city>:<category>:<price level>`):

```bash
redis-cli HDEL MapsLastSearchTime "united states:ca:los altos:eatery:0"
```
