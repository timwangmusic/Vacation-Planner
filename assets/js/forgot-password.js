import { sendDataXHR } from "./utils.js";

const form = document.querySelector('#password-reset-form');

form.addEventListener('submit', (event) => {
    event.preventDefault();

    const email = document.querySelector('#email').value;
    const url = `/v1/send-password-reset-email?email=${email}`;

    const XHR = new XMLHttpRequest();
    XHR.onload = () => {
        if (XHR.readyState === XHR.DONE) {
            if (XHR.status == 200) {
                $('#reset-password-email-success').removeClass('d-none');
            }
            if (XHR.status > 299) {
                $('#reset-password-email-error').removeClass('d-none');
            }
            if (XHR.status > 499) {
                console.log(XHR.response.error);
            }
        }
    }

    sendDataXHR(url, "GET", {}, XHR);
})

$('#password-reset-form').submit(async () => {
    const email = document.querySelector('#email').value;
    const url = `/v1/send-password-reset-email?email=${email}`;
    await axios.get(
        url
    ).catch(
        err => console.error(err)
    );
    return false;
})