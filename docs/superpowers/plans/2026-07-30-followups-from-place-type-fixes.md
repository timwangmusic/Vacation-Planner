# Follow-ups from the eatery place-type misclassification PR

Carried forward from the review of `fix/eatery-place-type-misclassification` (plan: `2026-07-29-eatery-place-type-fixes.md`). Everything here was reviewed, triaged, and deliberately deferred — none of it blocked that merge. Ordered by value.

## Important — deferred by explicit decision

### 1. `POI.AllPlaceCategories` — make the guard cover the invariant, not today's categories

**This is the highest-value item on the list.** The original incident survived because a guard test could not fail. Both current guards enumerate categories by hand — `test/place_category_test.go:41-44` and `iowrappers/nearby_search_validation_test.go:44-47` — so they protect the five categories that exist today rather than the invariant.

The final reviewer demonstrated this empirically: adding a `PlaceCategoryNightlife` with `GetPlaceTypes` returning `{"karaoke", "pub"}` (neither exists in the v1.7.0 SDK — the same species of mistake as `fast_food_restaurant`) plus a matching `GetPlaceCategory` case produced a **fully green suite**, build and all 6 packages. The incident is reproducible verbatim for any newly added category.

Fix: add `POI.AllPlaceCategories` and drive `ParsePlaceCategory` (`POI/categories.go:113-120`), `TestPlaceCategoryRoundTrip`, `TestCreateMapSearchRequestAcceptsKnownPlaceTypes`, `TestEncodeNearbySearchRedisKeyDistinct`, and the migration handler off it. A new category then lands inside every guard automatically. This converts "we fixed this bug" into "this bug shape cannot return."

### 2. `getNearbyPlacesByBrand` still truncates by prominence

`planner/planner.go:1298-1302` carries the identical false premise that the distance-sort task refuted for the category handler:

```go
places = iowrappers.Filter(places, func(place POI.Place) bool { return !place.KnownClosedOnDay(day) })
// Redis results are sorted by distance ascending; keep the nearest ones
if len(places) > limit { places = places[:limit] }
```

True on the cache path, false on the fresh path, where `PoiSearcher.NearbySearch` returns only `newPlaces` in Google prominence order (`iowrappers/poi_searcher.go:203-213`). A cold brand search can drop a 300m Dunkin' in favour of a 5km one.

Impact is ordering-only (brand searches use a single `LocationTypeAny` type, so no whole place types are lost), which is why it was deferred — but the repo now has one handler sorted and its sibling unsorted, carrying a comment this work explicitly falsified. Fix is one line: `iowrappers.SortPlacesByDistance(places, req.Location.Latitude, req.Location.Longitude)` before the truncation.

## Minor — migration robustness

### 3. Removals are not pipelined

`iowrappers/data_migrations.go:316` — reads were pipelined into batches of 100 but `ZRem` is still one round trip per removed member. The stated rationale for pipelining (a serial N+1 cannot finish inside Heroku's hard 30s H12) now covers only the read half. Irrelevant for the incident's ~17 rows; a bulk `apply=true` with thousands of hits re-enters the same ceiling.

### 4. Bucket sizes are only delivered if the run completes

`BucketSizes` / `TotalMembers` are measured up front (`data_migrations.go:270-277`) but serialized only in the terminal `ctx.JSON` (`planner/planner.go:303`). An H12 severs the request, so the operator who most needs the scale number is exactly the one who never receives it — `partial_report` covers returned errors, not a router timeout. A size-only mode (`?sizes=true`, returning right after the `ZCARD` loop) would make the property unconditional.

### 5. `getPlace` failure is indistinguishable from "no record exists"

`iowrappers/data_migrations.go:295-299`. A mid-run Redis fault silently skips every remaining member and returns a report reading `Misclassified: 0`, which an operator would reasonably read as "buckets are clean." Fail-safe in direction (nothing is deleted) but misleading. Distinguish `redis.Nil` from transport errors, or add a skipped/error count to the report.

## Minor — comment and doc precision

These matter more than usual: the write rule and the cleanup rule now **deliberately disagree**, and comments are most of what holds them apart. See "residual risk" below.

- `iowrappers/data_migrations.go:228-229` — the summary sentence still reads "removes places whose PRIMARY Google type does not belong to `cat`," which describes the *discarded* broad rule. Only the paragraph beneath it is accurate.
- `iowrappers/data_migrations.go:248-250` — describes `ReclassifyForCategory` as keeping "only when its primary type is one of the category's five search types." Omits its keep-on-no-`Types` branch (`POI/categories.go:161-163`), and "five" is Eatery-specific (Lodging has one).
- `test/redis_client_mocks/bucket_cleanup_test.go:101-102` — stale pre-existing comment still says untyped records are kept "matching `ReclassifyForCategory`'s keep-on-unknown rule," contradicting the deliberate-divergence doc added alongside it.
- `planner/planner.go:280-281` — the handler doc comment repeats the discarded rule in unqualified form; it is now the only place stating it that way.
- `2026-07-29-eatery-place-type-fixes.md:118` — the replacement grep invariant checks only `POI/categories.go` and only the `= LocationType("…")` declaration form. `TestPlaceCategoryRoundTrip` is the real guard, so this is cosmetic.

## Minor — pre-existing, untouched

- `iowrappers/nearby_search.go:196` — `maxRetries` equals the *total* category type count rather than the count actually attempted in a round, so a round where every active type fails but one sibling is skipped never reaches the cap. Bounded by `GoogleMapsSearchCallMaxCount = 5`, so no unbounded loop. Per-type failure tracking was explicitly ruled out of scope.
- `iowrappers/nearby_search.go:222` — the error log says "nearby search … failed" for a validation rejection where no search was attempted. Cheap fix, and worth doing because "fail loudly" is that code's whole promise.
- Malformed format verbs `%!s(<nil>)` at `planner/users.go:212,236` and `iowrappers/redis_client.go:497`.
- `iowrappers/data_migrations.go` hosts `SetPlace` / `AddGeoLocation`, which are generic `RedisClient` concerns; `redis_client.go` is their natural home. `AddGeoLocation` also widens the production API with a geo write that bypasses the type validation this PR added, and has no production caller.

## Residual risk to keep in mind

**The write rule and the cleanup rule now legitimately disagree.** The write path (`iowrappers/redis_client.go:225-245`) keys on the stamped `LocationType`; the cleanup rule keys on Google's *primary* type and deliberately keeps unmapped primaries (`meal_delivery`, `night_club`, empty `Types`) because the write path would legitimately place them there. A future refactor that "unifies" the two would reintroduce the incident.

The dangerous direction is test-pinned: reverting the migration to `ReclassifyForCategory` fails `TestRemoveMisclassifiedPlacesPrimaryTypeTruthTable`. What is *not* pinned is someone changing `ReclassifyForCategory` itself, or collapsing both into a single shared helper. That is why the comment-precision items above are load-bearing rather than cosmetic.

## Context worth not relearning

- `POI.ReclassifyForCategory` has exactly **one** production caller, `planner/planner.go:1439` (the merchant endpoint). The trip-planning path — `planner/solver.go:532` → `matching.NearbySearchForCategory` → `matching.CreatePlace` — reads the same `placeIDs:eatery:level*` buckets and never reclassifies. Anything reasoning about "what the buckets contain" must account for both readers.
- `meal_delivery`, `night_club`, `liquor_store`, `convenience_store` are all legal legacy Places types (`maps@v1.7.0/types.go:257,264,253,227`) that Google routinely lists first in `types[]`.
- Legacy Nearby Search answers an unknown `?type=` by **ignoring the filter**, not by erroring. Places API (New) `searchNearby` rejects it with `INVALID_ARGUMENT` — see `2026-07-29-places-api-new-migration.md`.
