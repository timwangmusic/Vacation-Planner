import { sendDataXHR } from "./utils.js";

const form = document.querySelector('#set-new-password-form');

form.addEventListener('submit', (event) => {
    event.preventDefault();

    const url = new URL(document.URL);
    const email = url.searchParams.get('email');
    const code = url.searchParams.get('code');
    const newPassword = document.querySelector('#new-password').value;

    const data = {
        'email': email,
        'code': code,
        'new_password': newPassword,
    }

    const XHR = new XMLHttpRequest();
    XHR.onload = () => {
        if (XHR.readyState === XHR.DONE) {
            if (XHR.status == 200) {
                window.location = "/v1/log-in";
            }
            if (XHR.status > 299) {
                $('#reset-password-error-alert').text(`Password reset failed! ${XHR.response.error}`);
                $('#reset-password-error-alert').removeClass('d-none');
            }
            if (XHR.status > 499) {
                console.log(XHR.response.error);
            }
        }
    }

    const backendUrl = '/v1/reset-password-backend';
    sendDataXHR(backendUrl, 'PUT', data, XHR);
})
