// JS for plan_template.html

import { locateMe, locationAutocomplete, setDateToday } from "./utils.js";
import { logOut, updateUsername } from "./user.js";

const username = updateUsername();
const totalItemsCount = 10;
const itemsPerPage = 2;

setDateToday();

document.getElementById("autofill").addEventListener(
    "click", locateMe
)

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
    const lastColumnOfLastRow = document.querySelector('table tr:last-child td:last-child');

    let newRow = template.insertRow(-1);
    let newCell = newRow.insertCell(0);
    let newCell2 = newRow.insertCell(1);
    let newCell3 = newRow.insertCell(2);

    let select = document.createElement("select");
    let cafeOption = document.createElement("option");
    let attractionOption = document.createElement("option");

    if (template.rows.length > 2) {
        start = lastColumnOfLastRow.firstChild.value;
    }

    let startTime = hourDropdown(start);
    startTime.value = start;

    let endTime = hourDropdown(start);
    if (startTime.value) {
        endTime.value = end;
    }

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

function hourDropdown(start) {
    let select = document.createElement("select");
    select.classList.add("form-select");
    for (let hour = start; hour < 24; hour++) {
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
    // remove pagination buttons
    $('#pagination').hide();
    // hide timeout alert message
    $('#request-timeout-error-msg').addClass('d-none');
    // hide no result alert message
    $('#no-valid-plan-error-msg').addClass('d-none');
    // remove previous search results
    $('#tables').empty();
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
    const url = "/v1/customize?date=" + document.getElementById("datepicker").value.toString() + "&price=" + document.getElementById("price").value.toString() + "&size=" + totalItemsCount;
    console.log(`data about to send: ${JSON.stringify(data)}`);

    axios.post(
        url, JSON.stringify(data), {timeout: 10000}
    ).then(
        function (response) {
            console.log(response.data);
            parseResponse(response.data);
            $('#searchSpinner').addClass("visually-hidden");
            $('#no-valid-plan-error-msg').addClass('d-none');
        }
    ).catch(
        err => {
            console.error(err)
            $('#searchSpinner').addClass("visually-hidden");
            $('#no-valid-plan-error-msg').addClass('d-none');
            if (err.response?.status === 408) {
                $('#request-timeout-error-msg').removeClass('d-none');
            }
        }
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

function parseResponse(response) {
    console.log("Raw JSON response is", response);
    const plansCount = response["travel_plans"]?.length;

    $(function () {
        if (plansCount > 0) {
            createPlanResultTables(plansCount);
            $.each(
                response["travel_plans"], function (idx, plan) {
                    console.log("processing travel plan:", plan);
                    let planTableBody = $(`#plan-${idx} tbody`);
                    $.each(plan.places, function (_, place) {
                        let aTag = $('<a>', {
                            text: place.place_name,
                            href: place.url
                        });

                        let $timeDiv = $('<div>').addClass('d-flex').css('color', 'darkcyan');
                        $timeDiv.append($('<span>').text(place.place_icon_css_class).addClass('material-icons'));
                        $timeDiv.append($('<span>').text(place.start_time + ' - ' + place.end_time).addClass('mx-2'));

                        let $tr = $('<tr>').append(
                            $('<td>').append($timeDiv),
                            $('<td>').text('').append(aTag),
                        );
                        planTableBody.append($tr);
                    })
                }
            )
            // only shows the first 3 results
            $('#tables .table').slice(itemsPerPage).hide();
            $('#pagination').show();
            $('#pagination').pagination({

                // Total number of items to be paginated
                items: totalItemsCount,

                // Items allowed on a single page
                itemsOnPage: itemsPerPage,
                onPageClick: function (pageIdx) {
                    const itemsOnPage = 2;
                    $('#tables .table').hide()
                        .slice(itemsOnPage * (pageIdx - 1),
                            itemsOnPage + itemsOnPage * (pageIdx - 1)).show();
                }
            });
        } else {
            $('#no-valid-plan-error-msg').removeClass('d-none');
        }
    })
}

function createPlanResultTables(planCount) {
    for (let i = 0; i < planCount; i++) {
        let newTable = $('<table>')
            .addClass('table table-sm table-bordered')
            .attr('id', 'plan-' + i)
            .css('background', 'lightcyan')
            .css('table-layout', 'fixed');
        let headerRow = $('<tr>');
        headerRow.append($('<th>').text('Time'));
        headerRow.append($('<th>').text('Place'));

        newTable.append($('<thead>').append(headerRow));

        newTable.append($('<tbody>'));

        $('#tables').append(newTable);
    }
}

$('#profile').click(() => window.location = `/v1/profile?username=` + username);

// auto-completes location input
locationAutocomplete(jQuery);
