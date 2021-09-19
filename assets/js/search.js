function updateUsername() {
    const username = Cookies.get('Username');
    if (username) {
        console.log(`username is ${username}`);
    } else {
        console.log("user is not logged in.");
    }

    const jwtToken = Cookies.get("JWT");
    // if JWT is present then the session is still valid, otherwise JWT token will be removed
    if (jwtToken) {
        console.log(`Decoded JWT token ${jwtToken}`);
    } else {
        console.log("log in expired.");
    }

    if (username && jwtToken) {
        document.getElementById("login").style.display = "none";
        document.getElementById("signup").style.display = "none";

        const userNameElement = document.getElementById("username");
        userNameElement.innerText = username;
    }
}

updateUsername();

function locateMe() {
    async function success(location) {
        const latitude = location.coords.latitude;
        const longitude = location.coords.longitude;
        const date = new Date();

        console.log(latitude, longitude);

        await fetch("/v1/reverse-geocoding" + "?lat=" + latitude.toString() + "&lng=" + longitude.toString())
            .catch(error => console.log(error))
            .then(response => {
                if (response.ok) {
                    response.json().then
                        (
                            data => {
                                document.getElementById("location").value = data.results.city + ", " + data.results.country;
                                document.getElementById("datepicker").value = date.toISOString().substring(0, 10);
                            }
                        );
                } else {
                    console.log(response.statusText);
                }
            });
    }

    function error() {
    }

    if (navigator.geolocation) {
        navigator.geolocation.getCurrentPosition(success, error);
    }
}

document.querySelector('#autofill').addEventListener('click', locateMe);

const cities = [
    "San Jose",
    "San Diego",
    "San Francisco",
    "Los Angeles",
    "New York",
    "Chicago",
    "Houston",
    "Philadelphia",
    "Phoenix",
    "San Antonio",
    "Dallas",
    "Indianapolis",
    "Austin",
    "Columbus",
    "Baltimore",
    "Boston",
    "Seattle",
    "Washington",
    "Portland",
    "Las Vegas",
    "Paris",
    "Rome",
    "Vancouver",
    "New Delhi",
    "Beijing",
    "Shanghai",
];

const countries = [
    "USA",
    "Italy",
    "France",
    "Canada",
    "China",
    "India",
]

$(function () {
    $("#city").autocomplete({
        source: cities
    })

    $("#country").autocomplete({
        source: countries
    })
});
