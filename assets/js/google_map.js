/**
 * Run in the script mode by trip_plan_details_template.html.
 * Define initMap, and called by Google Map API.
 */

// codepoint from https://fonts.google.com/icons
const placeCategoryToIconCode = {
  Eatery: "\ue56c",
  Visit: "\uea52",
};

class LatLng {
  constructor(lat, lng) {
    this.lat = lat;
    this.lng = lng;
  }
}

class Label {
  constructor(
    text,
    color = "#ffffff",
    fontFamily = "Material Icons",
    fontSize = "18px"
  ) {
    this.text = text;
    this.color = color;
    this.fontFamily = fontFamily;
    this.fontSize = fontSize;
  }
}

// Called by Google Map API
async function initMap() {
  // adjust zoom to city level plus one
  const zoom = 11;
  const mapDiv = document.getElementById("googleMap");

  const plan = await getTravelPlan();
  const placeIds = plan.PlaceDetails.map((p) => p.ID);
  const latLngs = makeLatLngs(plan.LatLongs);
  const labels = makeMarkerLabels(plan.PlaceCategories);
  const options = makeOptions(placeIds, latLngs, labels);

  const { Map } = await google.maps.importLibrary("maps");
  const map = new Map(mapDiv, {
    zoom: zoom,
    center: findCenter(latLngs),
    mapId: "DEMO_MAP_ID",
  });

  await addMarkers(map, options);
}

function makeLatLngs(arr) {
  try {
    return arr.map((x) => new LatLng(x[0], x[1]));
  } catch (e) {
    console.log(e);
    return [];
  }
}

function makeMarkerLabels(placeCategories) {
  try {
    const labels = [];
    for (let cat of placeCategories) {
      let iconCode = utils.placeCategoryToIconCode[cat];
      labels.push(new Label(iconCode));
    }
    return labels;
  } catch (e) {
    console.log(e);
    return [];
  }
}

function makeOptions(ids, latLngs) {
  try {
    opts = [];
    for (let i = 0; i < latLngs.length; i++) {
      opts.push({
        id: ids[i],
        position: latLngs[i],
      });
    }
    return opts;
  } catch (e) {
    console.log(e);
    return [];
  }
}

function findCenter(latLngs) {
  const center = new LatLng(-25.344, 131.036);
  try {
    let centerLat = utils.mean(latLngs.map((x) => x.lat));
    let centerLng = utils.mean(latLngs.map((x) => x.lng));
    return new LatLng(centerLat, centerLng);
  } catch (e) {
    console.log(e);
    return center;
  }
}

function arithmeticMean(arr) {
  let sum = arr.reduce((a, b) => a + b, 0);
  return sum / arr.length;
}

async function addMarkers(map, cfgs) {
  const { Place } = await google.maps.importLibrary("places");
  const { AdvancedMarkerElement, PinElement } = await google.maps.importLibrary(
    "marker"
  );

  try {
    for (let cfg of cfgs) {
      await utils.createMarker(
        map,
        cfg,
        Place,
        AdvancedMarkerElement,
        PinElement
      );
    }
  } catch (e) {
    console.log(e);
  }
}

async function createMarker(
  map,
  cfg,
  Place,
  AdvancedMarkerElement,
  PinElement
) {
  const place = new Place({
    id: cfg.id,
  });

  await place.fetchFields({
    fields: ["svgIconMaskURI", "iconBackgroundColor"],
  });

  const pinElement = new PinElement({
    background: place.iconBackgroundColor,
    glyph: new URL(String(place.svgIconMaskURI)),
    borderColor: "#ffffff",
  });

  new AdvancedMarkerElement({
    map: map,
    content: pinElement.element,
    position: cfg.position,
  });
}

const utils = {
  mean: arithmeticMean,
  createMarker: createMarker,
  addMarkers: addMarkers,
  findCenter: findCenter,
  LatLng: LatLng,
  makeLatLngs: makeLatLngs,
  makeMarkerLabels: makeMarkerLabels,
  makeOptions: makeOptions,
  Label: Label,
  placeCategoryToIconCode: placeCategoryToIconCode,
};

module.exports = utils;
