var DEFAULT_DURATION_MS = 3000;
var CAPTURE_TIMEOUT_MS = 5000;
var CAPTURED_TIMEOUT_MS = 30000;
var CLEANUP_MARGIN_MS = 1000;

function uniqueScreenNames(screenNames) {
  var names = [];

  for (var i = 0; i < screenNames.length; i++) {
    var name = String(screenNames[i]);
    if (names.indexOf(name) === -1) {
      names.push(name);
    }
  }

  names.sort();
  return names;
}

function create(screenNames, durationMs) {
  var expected = uniqueScreenNames(screenNames || []);

  return {
    state: expected.length === 0 ? "aborted" : "capturing",
    expected: expected,
    captured: {},
    capturedCount: 0,
    durationMs: durationMs === undefined ? DEFAULT_DURATION_MS : durationMs
  };
}

function status(model) {
  return model.state;
}

function abort(model) {
  if (model.state === "aborted" || model.state === "done") {
    return false;
  }

  model.state = "aborted";
  return true;
}

function capture(model, screenName) {
  var name = String(screenName);

  if (model.state !== "capturing") {
    return false;
  }
  if (model.expected.indexOf(name) === -1) {
    abort(model);
    return false;
  }
  if (model.captured[name]) {
    return false;
  }

  model.captured[name] = true;
  model.capturedCount += 1;
  if (model.capturedCount === model.expected.length) {
    model.state = "captured";
  }
  return true;
}

function captureFailed(model, screenName) {
  if (model.state === "aborted" || model.state === "done") {
    return false;
  }

  abort(model);
  return true;
}

function screensMatch(model, screenNames) {
  var current = uniqueScreenNames(screenNames || []);
  var matches = current.length === model.expected.length;

  for (var i = 0; matches && i < current.length; i++) {
    matches = current[i] === model.expected[i];
  }

  if (!matches) {
    abort(model);
  }
  return matches;
}

function reveal(model) {
  if (model.state !== "captured") {
    return false;
  }

  model.state = "revealing";
  return true;
}

function watchdogTimeoutMs(model) {
  if (model.state === "capturing") {
    return CAPTURE_TIMEOUT_MS;
  }
  if (model.state === "captured") {
    return CAPTURED_TIMEOUT_MS;
  }
  if (model.state === "revealing") {
    return model.durationMs + CLEANUP_MARGIN_MS;
  }
  return 0;
}

function expireWatchdog(model) {
  if (watchdogTimeoutMs(model) === 0) {
    return false;
  }

  return abort(model);
}

function progress(model, elapsedMs) {
  if (model.state === "done") {
    return 1;
  }
  if (model.state !== "revealing") {
    return 0;
  }

  var elapsed = Math.max(0, elapsedMs);
  return Math.min(1, elapsed / model.durationMs);
}

function completeReveal(model) {
  if (model.state !== "revealing") {
    return false;
  }

  model.state = "done";
  return true;
}
