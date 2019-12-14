# Vacation planner
[![CircleCI](https://circleci.com/gh/weihesdlegend/Vacation-Planner/tree/master.svg?style=svg&circle-token=7f88a49fd72bbe5020c873e24bc5f8a6e47bad63)](https://circleci.com/gh/weihesdlegend/Vacation-Planner/tree/master)

## Motivation
* Planning for travel is essential for having a most enjoyable trip. 
Meanwhile, travel planning is time consuming and the results are sometimes frustrating. 
People typically rely on electronic maps and online resources to plan their trips. 
Querying for different routes are tedious at best, and often times it is hard to remember which sources and destinations have been researched.
When taking in time and financial constraints, this manual planning process quickly becomes mind-boggling.
* Our goal is to provide a service that helps travellers plan for their ideal vacations under financial budget or time constraint.
* The initial version (V1) APIs let users to enter travel destination (POI), date and how they would like to divide the day into slots, and the service provides travel plans for the user.
* The initial version only plans for one-day trips, and it ranks places with only POI information without personal preferences.

## REST API
* The Planning GET REST API takes in user request with destination and time info and responds with vacation plans.

    url: `http://localhost:10000/planning/v1?country=us&city=chicago&radius=20000&weekday=5`

  * `country`: string in English, country name
  * `city`: string in English, city name
  * `radius`: a non-negative integer, providing number too large results in travel time limit exceed error
  * `weekday`: a number in [0-6], indicating weekday index from Sunday to Saturday

## Installation (Mac)
* git clone the repository
* update Homebrew with `brew update`
* Install MongoDB using Homebrew
    + Follow the installation instructions in https://docs.mongodb.com/manual/tutorial/install-mongodb-on-os-x/
    + Give permission to write the database directory and enter your password if prompted
* Install Redis using Homebrew with `brew install redis`. If redis is already installed, consider execute`brew upgrade redis`

## Run REST server
* Obtain Google Maps API key and set the `MAPS_CLIENT_API_KEY`, `MONGODB_URI=:27017`,
`REDISCLOUD_URL=redis://localhost:6379` environment variables
* Start (in background) Redis service with `brew services start redis`
* Start (in background) MongoDB service with `mongod --fork --syslog`
* Execute `go run main/main.go` to start the REST server

## Future Releases
* Personalization
* Multi-city, multi-day planning


## System Integration and External Services
* Redis
* MongoDB
* Google Maps API


## Programming Languages
* Backend: Golang
