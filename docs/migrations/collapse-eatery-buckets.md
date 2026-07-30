# Migration: collapse the eatery price buckets

One-time merge of `placeIDs:eatery:level0..4` into a single `placeIDs:eatery` geo index, so every
category has exactly one bucket the way `placeIDs:visit` always has.

`GET /v1/migrate/union-eatery-buckets` — admin only, **dry-run unless `apply=true`**.

## Run this BEFORE deploying

The union is purely additive and invisible to the running code: it creates a key nothing reads
yet. Deploying first would instead point every eatery read at a key that does not exist and
trigger a global cold-search burst.

```
1. dry run          →  2. apply  →  3. deploy  →  4. verify  →  5. delete the level* keys
```

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

# 1. Dry run. Reports each source bucket's size, their total, and the exact resulting
#    member count (computed by reading members — a dry run writes nothing).
curl -s -H "Authorization: Bearer $ADMIN_JWT" \
  "$BASE/v1/migrate/union-eatery-buckets" | jq

# 2. Apply. expected_after and target_after must match.
curl -s -H "Authorization: Bearer $ADMIN_JWT" \
  "$BASE/v1/migrate/union-eatery-buckets?apply=true" | jq

# 3. Deploy.

# 4. Verify: a non-zero eatery count, which /stats/places could never report before
#    (it built "placeIDs:eatery", a key nothing wrote).
curl -s -H "Authorization: Bearer $ADMIN_JWT" "$BASE/stats/places" | jq

#    Spot-check that scores survived, against the place's own stored coordinates:
redis-cli GEOPOS placeIDs:eatery "<some-place-id>"
redis-cli GET "place_details:place_ID:<some-place-id>" | jq '.Location'

# 5. Only once the deploy is healthy:
redis-cli DEL placeIDs:eatery:level0 placeIDs:eatery:level1 placeIDs:eatery:level2 \
               placeIDs:eatery:level3 placeIDs:eatery:level4
```

`expected_after` is the deduped union size, so it will be **lower** than `source_total` whenever a
place appears in more than one price bucket. That is the intended outcome, not data loss.

## Marker fields are also changing

The same change re-keys `MapsLastSearchTime` from `<country>:<admin1>:<city>:…` to
`<cell>:<category>` (see [reclassify-buckets.md](reclassify-buckets.md#steps) for the format).
Old fields are simply never read again. Expect one cold search per occupied cell per category
after deploy; the geo buckets are untouched, so reads return the full member set immediately and
only the markers re-establish. `redis-cli DEL MapsLastSearchTime` afterwards is optional cleanup.
