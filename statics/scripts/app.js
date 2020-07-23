const weekdayMap = {
    Monday: 0,
    Tuesday: 1,
    Wednesday: 2,
    Thursday: 3,
    Friday: 4,
    Saturday: 5,
    Sunday: 6,
}

function query(event) {
    const city = document.getElementById("city").value;
    const country = document.getElementById("country").value;
    const weekday = document.getElementById("weekday").value;
    const distance = document.getElementById("distance").value;

    event.preventDefault();

    let searchData = new Map();
    searchData.set("city", city);
    searchData.set("country", country);
    searchData.set("weekday", weekdayMap[weekday]);
    console.log("weekdayMap[weekday]")
    searchData.set("radius", distance);
    searchData.set("numberResults", 5);

    let arr = [];

    for (let [k, v] of searchData.entries()) {
        const entry = k + "=" + v.toString();
        arr.push(entry);
    }

    const query = arr.join("&");
    window.location.href = "https://best-vacation-planner.herokuapp.com/planning/v1?" + query;
}

const mySearchButton = document.getElementById("search_button");
mySearchButton.addEventListener("click", query);
