# Vacation planner
## Idea and Design
* Create an app for travelers to find desirable route under budget constraint. 
* The user enters the budget, start and end date of the travel and major points of interest (POI).
* The initial version only considers route planning related to POI instead of considering dining preferences, etc. 
* Once confirmed the route, the app should be able to track the progress of the traveler and adjust route intelligently.

## Point of Interests (POI)
* In travel, we consider POI as places travelers spend quality time. As such, POI include the following categories. 
    * Visit: museums, art gallery, amusement park, etc
    * Eat: restaurants, cafe, coffee shops, etc
    * Stay: hotel, airbnb, etc
* We collect following information using Google maps API.
    * Place ID - string, uniquely identify a POI
    * Name - string, name of the POI
    * Address - string, address of the POI
    * Location - string, "lat,lng"
    * Hours - list of string, opening hours
    * photos - list of json, photo for POI
    
## Public APIs and Third Party Services
* For hotel / flight / car rental queries, use Expedia public API or similar services.
* For finding route on land, use Google Maps Directions API.
* For searching nearby places, use Google Maps Nearby Search API. Types of POI we can query are listed in maps/types.go.
## Programming Languages
* Backend: Golang
* Frontend: Javascript
