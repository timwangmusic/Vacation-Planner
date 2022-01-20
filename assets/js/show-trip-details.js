import { updateUsername } from "./user.js";
import { Place, View } from "./place.js";

const cardBodies = document.querySelectorAll("div.card div.card-body");
const myCarousel = document.querySelector('#carouselExampleIndicators');
const saveIcon = document.querySelector("i.bi-bookmark"); 
const saveTooltip = new bootstrap.Tooltip(saveIcon);
const profile = document.getElementById("profile");

myCarousel.addEventListener('slide.bs.carousel', function (evt) {
    // show new card body and hide the old one
    cardBodies[evt.from].classList.add("d-none");
    cardBodies[evt.to].classList.remove("d-none");
})

const username = updateUsername(); 

// save bookmark tooltip
saveTooltip.disable();
saveIcon.addEventListener('click', postPlanForUser)

async function postPlanForUser() {
    const url = `/v1/users/${username}/plans`
    const plan = await getPlan();

    try {
        console.log("Sending post request to", url)
        await fetch(
            url, {
            method: "POST",
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(planToView(plan))
            }
        )
    } catch (e) {
        console.log("Err when saving plan:", e) 
        return 
    }

    this.classList.remove("bi-bookmark")
    this.classList.add("bi-bookmark-check-fill")
    this.setAttribute("data-bs-original-title", "Saved!")
    let saveTooltip = new bootstrap.Tooltip(this)
    saveTooltip.enable();
    saveTooltip.show();
}

async function getPlan() {
    const planUrl = toPlanJSONURL(document.URL)

    return await fetch(
        planUrl, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json'
        },
    }
    )
        .then(response =>
            response.json()
        )
        .catch(
            err => console.log(err)
        )
}

function planToView(plan) {
    const url = new URL(document.URL);
    const planId = getPlanId(url);
    const travelDate = getTravelDate(url);
    const view = new View (
        plan.TravelDestination,
        travelDate, // YYYY-MM-DD
        planId, 
        new Date().toISOString(),
        []
    )
    for (const pDetail of plan.PlaceDetails) {
        const {
            Name: placeName, 
            FormattedAdress: address, 
            TimePeriod: timePeriod = '10 - 16', // TODO: use actual time period for each place
            URL: mapURL
        } = pDetail 
        const place = new Place(timePeriod, placeName, address, mapURL)
        view.places.push(place)
    }
    return view 
}

function getPlanId(url) {
    const results = url.pathname.split('/')
    return results[results.length-1]
}

function getTravelDate(url){
    const searchParams = new URLSearchParams(url.search);
    const date = searchParams.get("date");
    if (date) {
        return date 
    }
    return new Date().toISOString().split('T')[0] // YYYY-MM-DD, current date
}
function toPlanJSONURL(pageURL) {
    const url = new URL(pageURL)
    const searchParams = new URLSearchParams(url.search);
    searchParams.append('json_only', 'true')
    url.search = searchParams.toString() 
    return url.toString(); 
}

// Link to User Profile page
profile.addEventListener("click", () => window.location = `/v1/users/${username}/profile`);