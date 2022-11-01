// utility functions for the Vacation Planner front end

// get current location of the user
function locateMe() {
    async function success(location) {
        const latitude = location.coords.latitude;
        const longitude = location.coords.longitude;

        console.log(`latitude ${latitude} and longitude: ${longitude}`);

        const url = "/v1/reverse-geocoding"
        await axios.get(url, {
            params: {
                lat: latitude,
                lng: longitude
            }
        })
            .then(
                response => {
                    const reverseGeocodingResults = response.data.results;
                    document.getElementById("location").value = [reverseGeocodingResults.city, reverseGeocodingResults.admin_area_level_one, reverseGeocodingResults.country].join(", ");
                }
            ).catch(
                err => console.error(err)
            )
    }

    function error() {
    }

    if (navigator.geolocation) {
        navigator.geolocation.getCurrentPosition(success, error);
    }
}

function preciseLocation() {
    if (navigator.geolocation) {
        navigator.geolocation.getCurrentPosition(
            async function (location) {
                const latitude = location.coords.latitude;
                const longitude = location.coords.longitude;
                console.log(`latitude ${latitude} and longitude: ${longitude}`);
                document.getElementById("location").value = longitude.toString() + ', ' + latitude.toString();
                document.getElementById("precise-location-flag").value = "true";
            }, (error) => { console.log(error) }
        )
    }
}

// capitalize the first character in a string
function capitalizeFirstChar(str) {
    return str.charAt(0).toUpperCase() + str.slice(1)
}

// set today's date for a datepicker element
function setDateToday() {
    const today = new Date();
    console.log("today's date is: " + today);
    let month = today.getMonth() + 1;
    if (month < 10) {
        month = "0" + month.toString();
    }
    let day = today.getDate();
    if (day < 10) {
        day = "0" + day.toString();
    }
    document.getElementById("datepicker").value = [today.getFullYear(), month, day].join("-");
}

// location input autocomplete
function locationAutocomplete($) {
    $("#location").autocomplete(
        {
            source: function (request, response) {
                $.ajax(
                    {
                        url: "/v1/cities",
                        dataType: "json",
                        data: { term: request.term },
                        success: function (data) {
                            response($.map(data.results, function (location) {
                                if (location.region) {
                                    return [location.city, location.region, location.country].join(", ")
                                }
                                return [location.city, location.country].join(", ")
                            }))
                        }
                    }
                )
            },
            minLength: 2,
        }
    )
}

export { locateMe, setDateToday, preciseLocation, capitalizeFirstChar, locationAutocomplete }
