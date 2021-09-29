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
        const today = new Date();

        console.log(latitude, longitude);

        const url = "/v1/reverse-geocoding"
        await axios.get(url, {
            params: {
                lat: latitude,
                lng: longitude
            }
        })
            .then(
                response => {
                    document.getElementById("location").value = response.data.results.city + ", " + response.data.results.country;
                    let month = today.getMonth() + 1;
                    if (month < 10) {
                        month = "0" + month.toString();
                    }
                    document.getElementById("datepicker").value = [today.getFullYear(), month, today.getDate()].join("-");
                }
            ).catch(
                err => console.error(err)
            )
    }

    function error() {
    }

    if (navigator.geolocation) {
        navigator.geolocation.getCurrentPosition(success, error);
    }
}

document.querySelector('#autofill').addEventListener('click', locateMe);

const locationSearchInput = document.getElementById('location');
locationSearchInput.addEventListener(
    "keyup", (evt) => {
        if (evt.key == "Enter") {
            console.log("Pressed Enter in location input!")
            const spinner = document.getElementById("searchSpinner");
            const searchIcon = document.getElementById("searchIcon");
            spinner.classList.remove("visually-hidden");
            searchIcon.classList.add("visually-hidden");
        }    
    }
)

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
