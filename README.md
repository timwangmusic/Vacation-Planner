# Vacation planner
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
* ```Planning(req *solution.PlanningRequest) (resp PlanningResponse)``` The Planning REST API takes in user request with destination and time info and response with suggested trip details.


## Future Releases
* Multi-city, multi-day vacation planning
* Personalization

## Third-party Libraries and External Services
* Logging: Logrus
* Redis Client: go-redis/redis
* MongoDB Client: globalsign/mgo
* Google Maps Places API to find detailed POI info 


## Programming Languages
* Backend: Golang
