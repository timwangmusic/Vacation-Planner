class View {
    constructor(destination, travelDate, id, createdDate, places) {
        this.destination = destination
        this.travel_date = travelDate
        this.original_plan_id = id
        this.created_at = createdDate 
        this.places = places
    }
  }

class Place{
    constructor(timePeriod, placeName, address, url) {
        this.time_period = timePeriod
        this.place_name = placeName
        this.address = address
        this.url = url
    }
}

export {View, Place}