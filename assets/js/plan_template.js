// JS for plan_template.html
import { logOut } from "./user.js";
import { setDateToday } from "./utils.js";

setDateToday();

document.getElementById("logout-confirm-btn").addEventListener(
    "click", logOut
)

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
    startTime.value = start;

    let endTime = hourDropdown();
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
    select.classList.add("form-select");
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

function rowToSlot(row) {
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

function tableToSlots() {
    const rows = tableToJSON()
    return rows.map(
        row => rowToSlot(row)
    )
}

async function postPlanTemplate() {
    document.getElementById("searchSpinner").classList.remove("visually-hidden");
    const location = document.getElementById('location').value.toString();
    const locationFields = location.split(",");
    locationFields.map(field => field.trim());
    let locationToPost = {}
    switch (locationFields.length) {
        case 2:
            locationToPost = {
                "city": locationFields[0],
                "country": locationFields[1]
            }
            break;
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
    axios.post(
        url, JSON.stringify(data)
    ).then(
        function (response) {
            console.log(response.data);
            parseResponse(response.data);
            $('#searchSpinner').addClass("visually-hidden");
            $('#no-valid-plan-error-msg').addClass('d-none');
        }
    ).catch(
        err => console.error(err)
    )
}

document.getElementById("add-row").addEventListener(
    "click", insertNewRow
);

document.getElementById("remove-row").addEventListener(
    "click", removeLastRow
);

document.getElementById("search").addEventListener(
    "click", postPlanTemplate
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
        const resultsTable = document.getElementById("results");
        if (response["travel_plans"]?.length > 0) {
            resultsTable.classList.remove("d-none");
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
        } else {
            resultsTable.classList.add("d-none");
            $('#no-valid-plan-error-msg').removeClass('d-none');
        }
    })
}
