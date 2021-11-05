import {updateUsername} from "./user.js";

let numberOfPlans = 5;
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
        body: JSON.stringify(planToView(sourcePlan, planIndex))
    }
    ).catch(
        err => console.error(err)
    )

    $(this).attr("disabled", "true");
    $(this).parent().attr("title", "saved!");

    $(`#edit-${planIndex}`).attr("disabled", "true");
    $(`#plan-table-${planIndex} tBody`).attr("contenteditable", "false");
}

function planToView(plan, planIndex) {
    const url = new URL(document.URL);

    const view = {
        destination: url.searchParams.get("location"),
        travel_date: url.searchParams.get("date"),
        original_plan_id: plan.id,
        created_at: new Date().toISOString(),
        places: []
    }

    $(`#plan-table-${planIndex} tbody tr`).map(function () {
        const $row = $(this);
        view.places.push(
            {
                "time_period": $row.find(`:nth-child(1) #interval-${planIndex}`).text().trim(),
                "place_name": $row.find(':nth-child(2)').find('a').text(),
                "address": $row.find(':nth-child(3)').text(),
                "url": $row.find(':nth-child(2)').find('a').attr("href")
            }
        )
    })

    return view;
}

// create button event actions
for (let planIndex = 0; planIndex < numberOfPlans; planIndex++) {
    $(`#save-${planIndex}`).click(postPlanForUser);
    $(`#edit-${planIndex}`).click(() => {
        $(`#plan-table-${planIndex} tBody`).attr("contenteditable", "true");
        $(this).innerText = "done";
    });
}
