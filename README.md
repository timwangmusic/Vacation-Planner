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

## REST API Endpoints
* Accessing the planning endpoints requires user login. A simple JWT-based mechanism is provided so that no session is stored on the server side.
    * To signup, go to `http://hostname/signup` and provide `username, email, password`
    * To login, go to `http://hostname/login` and provide `username, password`
* The Planning GET API endpoint takes user requests with destination, weekday and search radius info and responds with vacation plans in HTML.
The time slot schedule follows a template defined in the code base. Having a template simplifies the usage of the GET API.

    http verb: GET
    
    url: `http://hostname/planning/v1?country=us&city=chicago&radius=20000&weekday=5&numberResults=10`

  * `country`: string in English, country name
  * `city`: string in English, city name
  * `radius`: a non-negative integer, providing number too large results in travel time limit exceed error
  * `weekday`: an integer in [0-6], indicating weekday index from Sunday to Saturday
  * `numberResults`: a non-negative integer specifying number of desired plans. Defaults to 5 if 0 is provided.

 * The Planning POST API endpoint gives user more flexibility in configuring their day.
 Apart from specifying destination and weekday info, users can specify the start and end hours and the number of visit locations or eateries.
 Default values are provided for the additional parameters.
 
     http verb: POST
     
     url: `http://hostname/planning/v1`
 
   * `country`: string in English, country name
   * `city`: string in English, city name
   * `weekday`: an integer in [0-6], indicating weekday index from Sunday to Saturday
   * `start_time`: an integer in [0-23], indicating the starting hour of the day
   * `end_time`: an integer in [0-23], indicating the ending hour of the day, and we require `start_time < end_time`
   * `num_visit`: a non-negative integer, indicating the number of visit locations in each plan
   * `num_eatery`: a non-negative integer, indicating the number of eatery locations in each plan

## Installation (Mac)
* git clone the repository
* update Homebrew with `brew update`
* Install MongoDB using Homebrew
    + Follow the installation instructions in https://docs.mongodb.com/manual/tutorial/install-mongodb-on-os-x/
    + Give permission to write the database directory and enter your password if prompted
* Install Redis using Homebrew with `brew install redis`. If redis is already installed, consider execute`brew upgrade redis`

## Run REST server locally for development
* Obtain Google Maps API key and set the `MAPS_CLIENT_API_KEY`, `MONGODB_URI=:27017`,
`REDISCLOUD_URL=redis://localhost:6379` environment variables
* Start (in background) Redis service with `brew services start redis`
* Start (in background) MongoDB service with `mongod --fork --syslog`
* Execute `go run main/main.go` to start the server


## Production Deployment
* The service can be deployed on any service platform.
Particularly we have configured the code base and been deploying the service to Heroku.
* For deployment to Heroku, simply execute `git push heroku master` 


## Future Development Plans
* Personalization
* Multi-city, multi-day planning


## System Integration and External Services
* Redis
* MongoDB
* Google Maps API


## Programming Languages and Frameworks
* Backend: Golang
