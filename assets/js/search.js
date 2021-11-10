// methods for the search page
import { logOut, updateUsername } from "./user.js";

document.getElementById("logout-confirm-btn").addEventListener(
    "click", logOut
)

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

const username = updateUsername();

setDateToday();

document.querySelector('#autofill').addEventListener('click', locateMe);

const locationSearchInput = document.getElementById('location');
const spinner = document.getElementById("searchSpinner");
locationSearchInput.addEventListener(
    "keyup", (evt) => {
        if (evt.key === "Enter") {
            console.log("Pressed Enter in location input!")
            spinner.classList.remove("visually-hidden");
        }
    }
)

document.getElementById("profile").addEventListener("click", () => window.location = `/v1/users/${username}/profile`);
