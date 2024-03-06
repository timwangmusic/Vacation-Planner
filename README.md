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
