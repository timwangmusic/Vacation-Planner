// methods for the search page
import {updateUsername, logOut} from "./user.js";

updateUsername();

document.getElementById("logout-confirm-btn").addEventListener(
    "click", logOut
)

function locateMe() {
    async function success(location) {
        const latitude = location.coords.latitude;
        const longitude = location.coords.longitude;
        const today = new Date();

        console.log(latitude, longitude);
        console.log(today);

        const url = "/v1/reverse-geocoding"
        await axios.get(url, {
            params: {
                lat: latitude,
                lng: longitude
            }
        })
            .then(
                response => {
                    document.getElementById("location").value = response.data.results.city + ", " + response.data.results.country;
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
