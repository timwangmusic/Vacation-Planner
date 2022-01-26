// JS for plan_template.html
import { logOut, updateUsername } from "./user.js";
document.getElementById("logout-confirm-btn").addEventListener(
    "click", logOut
)

const username = updateUsername();
document.getElementById("profile").addEventListener("click", () => window.location = `/v1/users/${username}/profile`);

function removeLastRow() {
    const table = document.getElementById("template");
    // prevent deleting table headers
    if (table.rows.length > 1) {
        table.deleteRow(-1);
    }
}

function insertNewRow(start = '8', end = '10', category = 'Visit') {
    const template = document.getElementById("template");
    let newRow = template.insertRow(-1);
    let newCell = newRow.insertCell(0);
    let newCell2 = newRow.insertCell(1);
    let newCell3 = newRow.insertCell(2);

    let select = document.createElement("select");
    let cafeOption = document.createElement("option");
    let attractionOption = document.createElement("option");

    let startTime = hourDropdown();
    startTime.classList.add("form-select");
    startTime.value = start;

    let endTime = hourDropdown();
    endTime.classList.add("form-select");
    endTime.value = end;

    cafeOption.text = "Eatery";
    attractionOption.text = "Visit";

    select.classList.add("form-select");
    select.add(cafeOption);
    select.add(attractionOption);
    select.value = category;

    newCell.append(select);

    newCell2.append(startTime);

    newCell3.append(endTime);
}

function hourDropdown() {
    let select = document.createElement("select");
    for (let hour = 8; hour < 20; hour++) {
        let option = document.createElement("option");
        option.text = hour.toString();
        select.add(option);
    }
    return select;
}

function tableToJSON() {
    return $("#template").tableToJSON(
        {
            extractor: function ($cellIdx, $cell) {
                return $cell.find("select").val()
            }
        }
    )
}

function tableToSlots() {
    const rows = tableToJSON()
    return rows.map(
        function (row) {
            return {
                "category": row["Category"],
                "time_slot": {
                    "slot": {
                        "start": parseInt(row["Start"]),
                        "end": parseInt(row["End"])
                    }
                }
            }
        }
    )
}

async function postTemplate() {
    const location = document.getElementById('location').value.toString();
    const locationFields = location.split(",");
    let locationToPost = {}
    switch (locationFields.length) {
        case 2:
            locationToPost = {
                "city": locationFields[0],
                "country": locationFields[1]
            }
        case 3:
            locationToPost = {
                "city": locationFields[0],
                "adminAreaLevelOne": locationFields[1],
                "country": locationFields[2]
            }
    }

    const data = {
        "location": locationToPost,
        "slots": tableToSlots()
    }
    const url = "/v1/customize";
    console.log(`data about to send: ${JSON.stringify(data)}`);
    await fetch(
        url, {
        method: "POST",
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(data)
    }
    ).then(response => response.json())
        .then(
            response => parseResponse(response)
        )
        .catch(
            err => console.error(err)
        )
}


document.getElementById("add-row").addEventListener(
    "click", insertNewRow
);

document.getElementById("remove-row").addEventListener(
    "click", removeLastRow
);

document.getElementById("submit").addEventListener(
    "click", postTemplate
)

document.addEventListener("DOMContentLoaded", () => {
    // set a default time-category template
    insertNewRow(8, 11, 'Visit');
    insertNewRow(11, 13, 'Eatery');
    insertNewRow(13, 17, 'Visit');
});

async function parseResponse(response) {
    console.log("Raw JSON response is", response);

    $(function () {
        if (response["travel_plans"].length > 0) {
            let plan = response["travel_plans"][0];
            const newTableBody = document.createElement('tbody');

            $.each(plan.places, function (_placeIdx, place) {
                let aTag = $('<a>', {
                    text: place.place_name,
                    href: place.url
                });
                let $tr = $('<tr>').append(
                    $('<td>').text(place.start_time + " - " + place.end_time),
                    $('<td>').text('').append(aTag),
                );
                $tr.appendTo(newTableBody);
            })

            const oldTableBody = document.getElementById('results-table-body');
            oldTableBody.parentNode.replaceChild(newTableBody, oldTableBody);
            newTableBody.id = 'results-table-body';
        }
    })
}
