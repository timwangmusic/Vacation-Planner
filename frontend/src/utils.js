import $ from "jquery";

export function planningRequest() {
  const locationFields = splitLocationFields();
  if (locationFields.invalid) {
    return { invalid: true };
  }

  const data = {
    location: splitLocationFields(),
    slots: requestTableTimeSlots(),
    invalid: false,
  };
  return data;
}

function splitLocationFields() {
  const location = $("#search-bar-location").val();
  const locationFields = location.split(",");
  locationFields.map((field) => field.trim());
  var locationToPost = {};
  switch (locationFields.length) {
    case 2:
      locationToPost = {
        city: locationFields[0],
        country: locationFields[1],
        invalid: false,
      };
      break;
    case 3:
      locationToPost = {
        city: locationFields[0],
        adminAreaLevelOne: locationFields[1],
        country: locationFields[2],
        invalid: false,
      };
      break;
    default:
      locationToPost = {
        invalid: true,
      };
  }
  return locationToPost;
}

function requestTableTimeSlots() {
  var slots = [];
  $("#request-table tbody")
    .children("tr")
    .each(function (_idx, row) {
      var r = {};
      $(row)
        .children("td")
        .children("select")
        .each(function (_idx, s) {
          r[s.options[0].text] = s.options[s.selectedIndex].text;
        });
      slots.push(rowToSlot(r));
    });
  return slots;
}

function rowToSlot(row) {
  return {
    category: row["category"],
    time_slot: {
      slot: {
        start: parseInt(row["start"]),
        end: parseInt(row["end"]),
      },
    },
  };
}
