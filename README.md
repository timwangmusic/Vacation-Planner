# Unwind

+ Website: www.unwind.dev

![Unwind Logo](https://github.com/weihesdlegend/Vacation-Planner/blob/pr_logo/planner-app/public/logo.png?raw=true)

## Motivation
* Planning for travel is essential for having a most enjoyable trip.
Meanwhile, travel planning is time-consuming, and the results are sometimes unsatisfactory.
People typically rely on electronic maps and online resources to plan their trips.
Querying for different routes are tedious at best, and often times it is hard to remember which sources and destinations have been researched.
When taking in time and financial constraints, this manual planning process quickly becomes mind-boggling.
* Our goal is to provide a service for travellers to plan for their ideal vacations under financial or time budget.
* The planning APIs let users to enter travel destination (POI), date and how they would like to divide the day into slots, and the service provides travel plans for the user.
* The initial version only plans for single-day trips, and it ranks results without personal preferences.

## Features
* Save your favorite plans in your profile
* View trip details
* Make a plan yourself by creating a template

## Installation (Mac)
* git clone the repository
* update Homebrew with `brew update`
* Install Redis using Homebrew with `brew install redis`. If redis is already installed, execute `brew upgrade redis`


## Development
* Obtain Google Maps API key and set the `MAPS_CLIENT_API_KEY=YOUR_GCP_API_KEY`,
`REDISCLOUD_URL=redis://localhost:6379` environment variables
* Start (in background) Redis service with `brew services start redis`
* Execute `go run main/main.go` to start the server


## Production Deployment
* The service can be deployed on any service platform.
Particularly we have configured the code base and been deploying the service to Heroku.
* For deployment to Heroku, simply execute `git push heroku master` 


## Future Development Plans
* Multi-city, multi-day planning


## System Integration and External Services
* Redis
* Google Maps API


## Tech Stack
* Backend: Golang
* Frontend: Bootstrap and Javascript
