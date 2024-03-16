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
