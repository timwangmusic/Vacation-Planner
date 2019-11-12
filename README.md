# Vacation planner
![CircleCI](https://circleci.com/gh/weihesdlegend/Vacation-Planner.svg?style=svg&circle-token=7f88a49fd72bbe5020c873e24bc5f8a6e47bad63)

## Motivation
* Planning for travel is essential for having a most enjoyable trip. 
Meanwhile, travel planning is time consuming and the results are sometimes frustrating. 
People typically rely on electronic maps and online resources to plan their trips. 
Querying for different routes are tedious at best, and often times it is hard to remember which sources and destinations have been researched.
When taking in time and financial constraints, this manual planning process quickly becomes mind-boggling.
* Our goal is to provide a service that helps travellers plan for their ideal vacations under financial budget or time constraint.
* The initial version, which we will release soon, asks users to enter travel destination (POI), date and how they would like to divide the day into slots.
The service then suggest several travel plans for the user.
* The initial version only plans for one-day trip, and when selecting places it only considers POI information without personal dining preferences, etc.

## REST API
* The Planning REST API takes in user request with destination and time info and response with suggested trip details.

    `url: http://hostname/planning/country/city/search_radius`


## Installation (Mac)
* git clone the repository
* update Homebrew `brew update`
* Install MongoDB using Homebrew with `brew install mongodb`. The database data by default locates in `/data/db`.
You need to give permission to write the directory using `sudo chown -R id -un /data/db` and enter your password.
* Install Redis using Homebrew with `brew install redis`

## Run REST server
* Obtain Google Maps API key and modify `maps_client_api_key` field in `server_config.yml`.
* Start (in background) Redis service with `brew services start redis`
* Start (in background) MongoDB service with `mongod --fork --syslog`
* Execute `go run main/run_server.go` to start the REST server.

## Future Releases
* Multi-city, multi-day vacation planning
* Personalization


## Third-party Libraries and External Services
* Logging: Logrus
* Redis Client: go-redis/redis
* MongoDB Client: globalsign/mgo
* Google Maps Geocoding/Places API


## Programming Languages
* Backend: Golang
