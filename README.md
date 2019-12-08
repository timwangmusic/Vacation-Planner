# Vacation planner
[![CircleCI](https://circleci.com/gh/weihesdlegend/Vacation-Planner/tree/master.svg?style=svg&circle-token=7f88a49fd72bbe5020c873e24bc5f8a6e47bad63)](https://circleci.com/gh/weihesdlegend/Vacation-Planner/tree/master)

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

    `http://localhost:10000/planning/v1?/country=us&city=chicago&radius=20000&weekday=5`


## Installation (Mac)
* git clone the repository
* update Homebrew `brew update`
* Install MongoDB using Homebrew
    + Follow the latest instructions at - https://docs.mongodb.com/manual/tutorial/install-mongodb-on-os-x/
    + After downloading MongoDB, optionally create the **DB** directory. This is where the Mongo data files will live
* Run the Mongo daemon, in one of your terminal windows do **brew services start mongodb-community**. This should start the Mongo server
* You need to give permission to write the directory using `sudo chmod 0755 /data/db && sudo chown $USER /data/db` and enter your password if prompted
* Install Redis using Homebrew with `brew install redis`, if redis is already present, consider execute`brew upgrade redis`

## Run REST server
* Obtain Google Maps API key and set the `MAPS_CLIENT_API_KEY`, `MONGODB_URI=:27017`,
`REDISCLOUD_URL=redis://localhost:6379` environment variables.
* Start (in background) Redis service with `brew services start redis`
* Start (in background) MongoDB service with `mongod --fork --syslog`
* Execute `go run main/run_server.go` to start the REST server and query the REST API link above to check the http response

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
