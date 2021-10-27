import jwt_decode from "./jwt-decode.js";

let numberOfPlans = 5;

function updateUsername() {
    const jwt = Cookies.get("JWT");
    let username = "guest";

    if (jwt) {
        console.log("The JWT token is: ", jwt);

        const decodedJWT = jwt_decode(jwt);

        username = decodedJWT.username;
        console.log(`The current Logged-in username is ${decodedJWT.username}`)
    } else {
        console.log("The session has expired or the user is not logged in.");
    }

    const userProfileElement = document.getElementById("user-profile");

    userProfileElement.innerText = username;
    return username;
}

let username = updateUsername();

async function getPlans() {
    const plansUrl = document.URL + "&json_only=true";

    return await fetch(
        plansUrl, {
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

async function postPlanForUser() {
    const url = `/v1/users/${username}/plans`
    const fields = this.id.split("-");
    const planIndex = fields[fields.length - 1];

    const data = await getPlans();
    // the number of plans equals the array length in the JSON result
    numberOfPlans = data.length;
    const sourcePlan = data[planIndex];

    await fetch(
        url, {
        method: "POST",
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(planToView(sourcePlan))
    }
    ).catch(
        err => console.error(err)
    )

    const button = document.getElementById(this.id);
    button.setAttribute("disabled", "true");
    button.parentElement.setAttribute("title", "saved!")
}

function planToView(plan) {
    const url = new URL(document.URL);

    const view = {
        destination: url.searchParams.get("location"),
        travel_date: url.searchParams.get("date"),
        original_plan_id: plan.id,
        created_at: new Date().toISOString(),
        places: []
    }

    for (let place of plan.places) {
        view.places.push(
            {
                "place_name": place.place_name,
                "time_period": place.start_time + ' - ' + place.end_time,
                "address": place.address,
                "url": place.url
            }
        )
    }
    return view;
}

// create buttons
for (let i = 0; i < numberOfPlans; i++) {
    document.getElementById(`save-${i}`).onclick = postPlanForUser;
}

function initializeToolTips() {
    const tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
    tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl)
    });
}

// initializeToolTips();
