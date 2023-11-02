import jwt_decode from "./jwt-decode.js";

const updateUsername = function () {
  const jwt = Cookies.get("JWT");
  let username = "guest";

  if (jwt) {
    console.log("The JWT token is: ", jwt);

    const decodedJWT = jwt_decode(jwt);

    username = decodedJWT.username;
    console.log(`The current Logged-in username is ${decodedJWT.username}`);
  } else {
    console.log("The session has expired or the user is not logged in.");
    // Hide logout and profile dropdown items when user is not logged in
    document.getElementById("logout-button-item").classList.add("d-none");
    document.getElementById("profile").classList.add("d-none");
    // Display login dropdown item
    document.getElementById("login-button-item").classList.remove("d-none");
    return username;
  }

  // Hide signup link when user is already logged in
  const signUpLink = document.getElementById("signup");
  if (signUpLink) {
    document.getElementById("signup").style.display = "none";
  }

  const userProfileElement = document.getElementById("user-profile");

  userProfileElement.innerText = username;
  return username;
};

function logOut() {
  const cookieToRemove = "JWT";
  const jwt = Cookies.get(cookieToRemove);
  if (jwt === null) {
    console.error("JWT does not exist");
    return;
  }
  console.log(`JWT ${cookieToRemove} is removed`);
  Cookies.remove(cookieToRemove, { path: "/v1" });
  location.reload();
}

const username = updateUsername();

export { updateUsername, logOut };
