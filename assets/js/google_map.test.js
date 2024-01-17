const utils = require("./google_map");
const { google } = require("./__mock__/maps");
const {
  test,
  expect,
  describe,
  beforeEach,
  afterEach,
  beforeAll,
  afterAll,
} = require("@jest/globals");
const { LatLng, Label } = require("./google_map");
const origGoogle = global.google;

beforeAll(() => {
  global.google = google; // mock
});

afterAll(() => {
  global.google = origGoogle; // restore
});

describe("Test middle point", () => {
  const { mean: arithmeticMean } = utils;
  test("Middle of an integer array [1,2,3]", () => {
    let data = [1, 3, 4];
    let result = 8 / 3;
    expect(arithmeticMean(data)).toBeCloseTo(result);
  });

  test("Middle of an empty array", () => {
    let data = [];
    expect(arithmeticMean(data)).toBeNaN();
  });
});

describe("Test Single Marker", () => {
  const { createMarker, Label } = utils;
  test("Create a Google map marker", () => {
    let data = {
      map: "test",
      options: {
        position: [0, 1],
        label: new Label("\uea52"),
      },
    };

    google.maps.Marker = jest.fn();
    let mock = google.maps.Marker;
    let mockArgs = {
      map: data.map,
      ...data.options,
    };

    // test target func
    createMarker(data.map, data.options);

    // verify
    expect(mock).toHaveBeenCalledTimes(1);
    expect(mock).toHaveBeenCalledWith(mockArgs);

    mock.mockRestore();
  });
});

describe("Test Google map markers", () => {
  const { addMarkers } = utils;
  beforeEach(() => {
    jest.spyOn(utils, "createMarker");
  });
  afterEach(() => {
    utils.createMarker.mockRestore();
  });

  test("Successfully add markers", () => {
    const spy = utils.createMarker;
    spy.mockImplementation();
    let data = {
      map: "test",
      cfgs: ["cfg0", "cfg1", "cfg2"],
    };
    let args = [];
    for (let cfg of data.cfgs) {
      args.push([data.map, cfg]);
    }

    // test target func
    addMarkers(data.map, data.cfgs);

    // verify
    expect(spy).toHaveBeenCalledTimes(data.cfgs.length);
    for (let i = 0; i < args.length; i++) {
      expect(spy).toHaveBeenNthCalledWith(i + 1, ...args[i]);
    }
  });

  test("Fail to add markers", () => {
    // setup
    const msg = "fake error";
    const fakeErr = new Error(msg);
    utils.createMarker.mockImplementation(() => {
      throw fakeErr;
    });
    const spy = jest.spyOn(console, "log");
    spy.mockImplementation();
    let data = {
      map: "test",
      cfgs: ["cfg0", "cfg1", "cfg2"],
    };

    // test target func
    addMarkers(data.map, data.cfgs);

    // verify
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith(fakeErr);

    // restore
    spy.mockRestore();
  });
});

describe("Test findCenter of array of latLng points", () => {
  const { findCenter, LatLng } = utils;
  beforeEach(() => {
    jest.spyOn(utils, "mean");
  });
  afterEach(() => {
    utils.mean.mockRestore();
  });
  test("Successfully find center", () => {
    // setup
    const latCenter = 100;
    const lngCenter = 200;
    let data = [new LatLng(50, 150), new LatLng(150, 250)];
    const spy = utils.mean;
    spy
      .mockReturnValue(0)
      .mockReturnValueOnce(latCenter)
      .mockReturnValue(lngCenter);

    // test
    const result = findCenter(data);

    // verify
    const target = new LatLng(latCenter, lngCenter);
    expect(result).toEqual(target);
  });

  test("Fail to find center", () => {
    // setup
    const fakeErr = new Error("fake error");
    utils.mean.mockImplementation(() => {
      throw fakeErr;
    });
    const spy = jest.spyOn(console, "log");
    spy.mockImplementation();
    let data = [new LatLng(0, 0), new LatLng(1, 1)];

    // test
    const result = findCenter(data);

    // verify
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith(fakeErr);
    const defaultTarget = new LatLng(-25.344, 131.036);
    expect(result).toEqual(defaultTarget);

    // restore
    spy.mockRestore();
  });
});

describe("Test to create an array of LatLng", () => {
  const { makeLatLngs } = utils;
  test("Succesfully create new Latlngs", () => {
    // setup
    const data = [
      [0, 0],
      [50, 150],
      [150, 250],
    ];
    const target = [];
    for (let x of data) {
      target.push(new LatLng(x[0], x[1]));
    }

    // test
    const result = makeLatLngs(data);

    // verify
    expect(result).toEqual(target);
  });

  test("Fail to create new LatLngs", () => {
    // setup
    const fakeErr = new Error("fake error from Rui");
    const spyArrMap = jest.spyOn(Array.prototype, "map");
    spyArrMap.mockImplementation(() => {
      throw fakeErr;
    });
    const spy = jest.spyOn(console, "log");
    spy.mockImplementation();
    const data = [
      [0, 0],
      [50, 150],
      [150, 250],
    ];

    // test
    const result = makeLatLngs(data);

    // verify
    expect(spyArrMap).toBeCalledTimes(1);
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith(fakeErr);
    expect(result).toEqual([]);

    // restore
    spyArrMap.mockRestore();
    spy.mockRestore();
  });
});

describe("Test to create an array of marker labels", () => {
  const { makeMarkerLabels, Labels } = utils;
  const origMap = utils.placeCategoryToIconCode;
  const fakeMap = { a: "code0", b: "code1" };
  beforeEach(() => {
    utils.placeCategoryToIconCode = fakeMap;
  });
  afterEach(() => {
    utils.placeCategoryToIconCode = origMap;
  });
  test("Successfully create new labels", () => {
    // setup
    const categories = ["a", "b", "a"];
    const target = categories.map((x) => new Label(fakeMap[x]));

    // test
    const result = makeMarkerLabels(categories);

    // verify
    expect(result).toEqual(target);
  });
});

describe("Test to create an array of marker options", () => {
  const { makeOptions } = utils;
  test("Successfully create an array of marker options", () => {
    // setup
    const latlngs = [0, 1];
    const labels = ["a", "b"];
    const target = [
      { position: 0, label: "a" },
      { position: 1, label: "b" },
    ];

    // test
    const result = makeOptions(latlngs, labels);
    expect(result).toEqual(target);
  });
});
