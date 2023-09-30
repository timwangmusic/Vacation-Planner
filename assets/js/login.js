import { sendDataXHR } from "./utils.js";

const form = document.getElementById("login-form");

form.addEventListener("submit", function (event) {
    event.preventDefault();

    const username = document.getElementById("username").value;
    const password = document.getElementById("password").value;

    const url = "/v1/login";

    const data = {
        username: username,
        password: password,
    }

    const XHR = new XMLHttpRequest();
    XHR.onload = function() {
        if (XHR.readyState === XHR.DONE) {
            if (XHR.status === 200) {
                window.location = "/";
            } else if (XHR.status === 401) {
                document.getElementById("log-in-error-alert").classList.remove("d-none");
            }
        }
    }

    sendDataXHR(url, "POST", data, XHR);
});
