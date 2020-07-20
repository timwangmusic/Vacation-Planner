function query(event) {
    const city = document.getElementById("city").value;
    const country = document.getElementById("country").value;
    const weekday = document.getElementById("weekday").value;
    const distance = document.getElementById("distance").value;

    event.preventDefault();

    let searchData = new Map();
    searchData.set("city", city);
    searchData.set("country", country);
    searchData.set("weekday", weekday);
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
