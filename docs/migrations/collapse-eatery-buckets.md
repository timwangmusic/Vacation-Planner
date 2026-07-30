# Migration: collapse the eatery price buckets

One-time merge of `placeIDs:eatery:level0..4` into a single `placeIDs:eatery` geo index, so every
category has exactly one bucket the way `placeIDs:visit` always has.

Two ways to run it:

- **`redis-cli ZUNIONSTORE`** — the pre-deploy path. One command, no application code involved.
- **`GET /v1/migrate/union-eatery-buckets`** — admin only, dry-run unless `apply=true`. Reports
  sizes and predicts the result, and is re-runnable. Only available *after* this change ships.

## Ordering

The union must land before the collapsed-key read goes live, or eatery reads hit a key that does
not exist yet and every request returns empty until organic traffic refills it.

But the HTTP endpoint **is part of the code being deployed**, so it cannot be the pre-deploy step
— it does not exist until the deploy that introduces it. Use `redis-cli` for that step:

```
1. redis-cli union  →  2. deploy  →  3. verify (endpoint dry run)  →  4. delete the level* keys
```

The union is purely additive and invisible to the running code — it creates a key nothing reads
yet — so step 1 is safe to run at any point beforehand.

If you deploy first by mistake it is recoverable, not fatal: the new write path populates
`placeIDs:eatery` organically and the union later merges the legacy members in (the target is
included in the union sources, so nothing is lost). The cost is degraded eatery results until
each area gets re-searched, plus the Google spend for those searches.

Note what the ordering does **not** buy you: the marker re-key (see below) forces one cold search
per occupied cell per category regardless of migration order. What running the union first
protects is the ~8.5k existing eatery members staying readable in the meantime.

## Why the split is going away

Eateries were filed under `placeIDs:eatery:level<N>` keyed on each place's own price level, while
reads asked for the level the *caller* wanted. Three things made that lossy:

- Google omits `price_level` for most places, so `res.PriceLevel == 0` and they nearly all landed
  in `level0`.
- Google only accepts a price filter at level ≥ 3, so searches for levels 0/1/2 issued
  **identical** requests — three cold fan-outs per area instead of one — then scattered the
  results across five buckets and each read back its own fifth.
- Price selection was already happening downstream anyway, in
  `matching.filterPlacesOnPriceLevel` (`planner/solver.go`).

Redis GEO is a sorted set scored by 52-bit geohash and `GEORADIUS` probes 9 geohash cells at
O(log N + M), so one bucket holds millions of members without degrading. The split bought nothing
and cost 5× fragmentation.

## `AGGREGATE MIN` is mandatory

A GEO member's score **is** its 52-bit geohash. `redis.ZStore.Aggregate` defaults to `SUM`, which
would add the scores of any place present in two source buckets and silently relocate it — in
practice into the ocean. The migration passes `MIN`, which keeps a real geohash; a place's
coordinates are identical across buckets, so which one survives does not matter.

`TestUnionEateryPriceBucketsPreservesCoordinates` pins this by seeding one place into two buckets
and asserting a 100 m `GEORADIUS` around its true coordinates still finds it.

The target key is included in the union sources, so the migration is re-runnable and cannot drop
members that already-deployed code has written to the collapsed key.

## Steps

```bash
BASE=https://best-vacation-planner.herokuapp.com
# Heroku: eval $(heroku config:get REDIS_URL -a <app>) or use `heroku redis:cli -a <app>`
R="redis-cli -u $REDIS_URL"

# --- 1. Union, before deploying. -------------------------------------------------
# Record the starting sizes so step 3 has something to check against.
for L in 0 1 2 3 4; do echo -n "level$L "; $R ZCARD "placeIDs:eatery:level$L"; done
$R ZCARD placeIDs:eatery   # expected 0 on a first run

# Note the SIX keys: the target is included so the command is idempotent and cannot
# drop members already written to the collapsed key. AGGREGATE MIN is mandatory.
$R ZUNIONSTORE placeIDs:eatery 6 \
  placeIDs:eatery:level0 placeIDs:eatery:level1 placeIDs:eatery:level2 \
  placeIDs:eatery:level3 placeIDs:eatery:level4 placeIDs:eatery AGGREGATE MIN

$R ZCARD placeIDs:eatery   # <= the sum above; lower means a place was in two buckets

# Spot-check that a geohash score survived, against that place's own record.
# These two must agree to within a few metres.
ID=$($R ZRANGE placeIDs:eatery 0 0 | head -1)
$R GEOPOS placeIDs:eatery "$ID"
$R GET "place_details:place_ID:$ID" | jq '.Location'

# --- 2. Deploy. -----------------------------------------------------------------

# --- 3. Verify. -----------------------------------------------------------------
# The endpoint exists now. A dry run re-reports the sizes and predicts the same count,
# which confirms the deployed code resolves the key you just populated.
curl -s -H "Authorization: Bearer $ADMIN_JWT" \
  "$BASE/v1/migrate/union-eatery-buckets" | jq

# A non-zero eatery count — which /stats/places could never report before, because it
# built "placeIDs:eatery", a key nothing wrote.
curl -s -H "Authorization: Bearer $ADMIN_JWT" "$BASE/stats/places" | jq

# --- 4. Only once the deploy is healthy. ----------------------------------------
$R DEL placeIDs:eatery:level0 placeIDs:eatery:level1 placeIDs:eatery:level2 \
       placeIDs:eatery:level3 placeIDs:eatery:level4
```

To roll back before step 4, `DEL placeIDs:eatery` — the legacy keys are untouched until then.

`expected_after` is the deduped union size, so it will be **lower** than `source_total` whenever a
place appears in more than one price bucket. That is the intended outcome, not data loss.

## Marker fields are also changing

The same change re-keys `MapsLastSearchTime` from `<country>:<admin1>:<city>:…` to
`<cell>:<category>` (see [reclassify-buckets.md](reclassify-buckets.md#steps) for the format).
Old fields are simply never read again. Expect one cold search per occupied cell per category
after deploy; the geo buckets are untouched, so reads return the full member set immediately and
only the markers re-establish. `redis-cli DEL MapsLastSearchTime` afterwards is optional cleanup.
