// methods for the search page
import { locateMe, preciseLocation, setDateToday, randomPriceRange } from "./utils.js";
import { logOut } from "./user.js";

setDateToday();

(function ($) {
    $("#location").autocomplete(
        {
            source: function (request, response) {
                $.ajax(
                    {
                        url: "/v1/cities",
                        dataType: "json",
                        data: { term: request.term },
                        success: function (data) {
                            response($.map(data.results, function (city) {
                                if (city.region) {
                                    return [city.city, city.region, city.country].join(", ")
                                }
                                return [city.city, city.country].join(", ")
                            }))
                        }
                    }
                )
            },
            minLength: 2,
        }
    )
})(jQuery);

document.getElementById("logout-confirm-btn").addEventListener(
    "click", logOut
)

const FORM = document.getElementById("main-form");
const STORAGE_ITEM = "location";
const LOCATION_INPUT = document.getElementById("location");
$(document).ready(() => {
    const val = sessionStorage.getItem(STORAGE_ITEM);
    if (val) {
        console.log(`Set the Location based on PageLoad...` + val);
        LOCATION_INPUT.value = val;
    }
});

FORM.addEventListener('submit', () => {
    if (LOCATION_INPUT.value) {
        sessionStorage.setItem(STORAGE_ITEM, LOCATION_INPUT.value);
        console.log(`The location is ${LOCATION_INPUT.value}`);
    }
})

document.querySelector('#autofill').addEventListener('click', locateMe);

document.querySelector('#use-precise-location').addEventListener('click', preciseLocation);

document.querySelector('#priceToSelect').addEventListener('change', randomPriceRange)

const locationSearchInput = document.getElementById('location');
const spinner = document.getElementById("searchSpinner");
locationSearchInput.addEventListener(
    "keyup", (evt) => {
        if (evt.key === "Enter") {
            console.log("Pressed Enter in location input!")
            spinner.classList.remove("visually-hidden");
        }
    }
);
// hide spinner when switching pages
const hideSpinner = function () {
    spinner.classList.add("visually-hidden");
}
document.addEventListener("visibilitychange", hideSpinner);

document.getElementById("profile").addEventListener("click", () => window.location = `/v1/users/${username}/profile`);
