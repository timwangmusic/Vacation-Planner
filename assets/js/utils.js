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

export{ locateMe, setDateToday }