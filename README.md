# Unwind

+ Website: www.unwind.dev

![Unwind Logo](assets/public/logo.png)

## Motivation

Travel planning is vital for a great vacation, but it can be a tedious and frustrating process. Sifting through maps and
countless online resources is not only time-consuming but often yields less-than-ideal results. Juggling time and budget
constraints adds another layer of complexity. Our goal is to streamline this process, empowering travelers to create
unforgettable vacations that fit their budgets and schedules. Our planning APIs simplify the experience by allowing
users to input their desired destination, date, and time preferences. The service then generates customized travel plans
tailored to their needs. While the initial version focuses on single-day trips and generic rankings, future iterations
will incorporate personalized recommendations.

## Features

* Save your favorite plans in your profile
* View trip details
* Make a plan yourself by creating a template

## Place Search API

Two endpoints let a caller add a place to the shared cache by name, as an alternative to the
category-based nearby-places search: search by free text, then confirm one result into the
cache.

### `POST /v1/place-search`

Runs a free-text Google Places search around a coordinate and returns every result as a
confirmable candidate; nothing is written to the shared cache by this call. Every result
(insertable or not) is stashed server-side under its Google place ID for 30 minutes so a
subsequent confirm call can resolve it by ID alone rather than trusting place data an HTTP
caller might send back.

Request:

```json
{
  "query": "Joe's Pizza",
  "location": {"latitude": 40.7309, "longitude": -74.0021},
  "radius": 5000,
  "limit": 10
}
```

`query` is required (2-120 characters). `location` (`latitude`/`longitude`) is required, with
no default — unlike nearby search, an unbiased text query like "konjoe" can resolve to the
wrong continent without a coordinate to anchor it. `radius` is in meters and is clamped to the
service's max search radius (16,000 m / ~10 miles) when zero or larger. `limit` defaults to 10
and is capped at 20.

Response (`200`, fields elided for brevity):

```json
{
  "results": [
    {
      "place": {
        "ID": "ChIJd8BlQ2BZwokRAFUEcm_qrcA",
        "Name": "Joe's Pizza",
        "Status": "OPERATIONAL",
        "LocationType": "restaurant",
        "Types": ["restaurant", "food", "point_of_interest", "establishment"],
        "FormattedAddress": "7 Carmine St, New York, NY 10014",
        "Location": {"latitude": 40.7309, "longitude": -74.0021, "city": "", "adminAreaLevelOne": "", "country": ""},
        "PriceLevel": 1,
        "Rating": 4.5,
        "UserRatingsTotal": 3200
      },
      "category": "Eatery",
      "insertable": true
    }
  ]
}
```

`category` and `insertable` are always derived server-side from the place's own Google types.
A candidate whose primary type does not map to a known category is still returned (so the
caller can see it), but with `category: ""` and `insertable: false`.

### `POST /v1/place-search/confirm`

Inserts a previously returned candidate into the shared cache — `placeIDs:<category>` plus a
`place_details:place_ID:*` record — making one Place Details call to fill in hours, address,
URL, and summary before writing.

Request:

```json
{"placeId": "ChIJd8BlQ2BZwokRAFUEcm_qrcA"}
```

Response (`200`):

```json
{
  "place": { "ID": "ChIJd8BlQ2BZwokRAFUEcm_qrcA", "...": "same shape as place-search's place object, now enriched with hours/URL/summary" },
  "category": "Eatery",
  "alreadyCached": false
}
```

Error responses:

* `404` `{"error": "...", "code": "candidate_expired"}` — the place ID was never searched, or
  its 30-minute stash entry expired.
* `422` `{"error": "...", "code": "unsupported_place_type", "placeType": "<google type>"}` —
  the candidate's primary Google type does not map to any category; nothing is written.

### Authentication

Both endpoints require the same authentication as the other `/v1` endpoints: a Personal
Access Token via `Authorization: Bearer <token>`, or a JWT session cookie as a browser
fallback.

### Safety design

The category a place lands under is always computed server-side from Google's own primary
type on the place, never accepted from the caller, and a primary type that maps to no known
category is refused outright rather than defaulted into some bucket — so nothing
client-supplied ever reaches the shared place cache unclassified.

## Installation (Mac)

* git clone the repository
* update Homebrew with `brew update`
* Install Redis using Homebrew with `brew install redis`
    * If Redis is already installed, execute `brew upgrade redis`

## Development

* Obtain Google Maps API key and set the `MAPS_CLIENT_API_KEY=YOUR_GCP_API_KEY`,
  `REDISCLOUD_URL=redis://localhost:6379` environment variables.
* Set environment variable `ENVIRONMENT=DEVELOPMENT,SENDGRID_API_KEY=NO_KEY` as we do not create mailers in development
  environment.
* Start (in background) Redis service with `brew services start redis`.
* Execute `go run main/main.go` to start the server.

## Running with Docker Compose

* Use command `docker-compose up -d` to start the containers.
* Make sure to set up environment variables `REDIS_URL=redis://redis:6379` and `MAPS_CLIENT_API_KEY=YOUR_GCP_API_KEY`.
  Note that using `localhost` for redis URL does not work.
* To stop the containers, use the command `docker-compose stop`.

## Production Deployment

* The service can be deployed on any service platform.
  Particularly we have configured the code base and been deploying the service to Heroku.
* For deployment to Heroku, simply execute `git push heroku master`.

## Future Development Plans

* Multi-city, multi-day planning

## System Integration and External Services

* Redis
* Google Maps API
* GeoNames Web Services

## Tech Stack

* Backend: Golang
* Frontend: Bootstrap and Javascript

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=timwangmusic/Vacation-planner&type=Date)](https://star-history.com/#timwangmusic/Vacation-planner&Date)
