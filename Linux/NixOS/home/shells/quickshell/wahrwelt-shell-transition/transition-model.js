var DEFAULT_OUTGOING_DURATION_MS = 3000;
var DEFAULT_BRIDGE_DURATION_MS = 4000;
var DEFAULT_INCOMING_DURATION_MS = 3000;
var CAPTURE_TIMEOUT_MS = 5000;
var CAPTURED_TIMEOUT_MS = 1000;
var CLEANUP_MARGIN_MS = 750;

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

function create(screenNames) {
  var expected = uniqueScreenNames(screenNames || []);
  var totalDurationMs = DEFAULT_OUTGOING_DURATION_MS
    + DEFAULT_BRIDGE_DURATION_MS
    + DEFAULT_INCOMING_DURATION_MS;

  return {
    state: expected.length === 0 ? "aborted" : "capturing",
    expected: expected,
    captured: {},
    capturedCount: 0,
    coverPresented: {},
    coverPresentedCount: 0,
    settlingPresented: {},
    outgoingDurationMs: DEFAULT_OUTGOING_DURATION_MS,
    bridgeDurationMs: DEFAULT_BRIDGE_DURATION_MS,
    incomingDurationMs: DEFAULT_INCOMING_DURATION_MS,
    totalDurationMs: totalDurationMs
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
  if (model.state === "covered" || model.state === "incoming" || model.state === "settling") {
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

function beginTransition(model) {
  if (model.state !== "captured") {
    return false;
  }

  model.state = "outgoing";
  model.coverPresented = {};
  model.coverPresentedCount = 0;
  return true;
}

function coverFramePresented(model, screenName) {
  var name = String(screenName);
  var count;

  if (model.state !== "outgoing") {
    return false;
  }
  if (model.expected.indexOf(name) === -1) {
    abort(model);
    return false;
  }
  count = model.coverPresented[name] || 0;
  if (count >= 2) {
    return false;
  }

  model.coverPresented[name] = count + 1;
  if (model.coverPresented[name] === 2) {
    model.coverPresentedCount += 1;
  }
  if (model.coverPresentedCount === model.expected.length) {
    model.state = "covered";
  }
  return true;
}

function beginIncoming(model) {
  if (model.state !== "covered") {
    return false;
  }

  model.state = "incoming";
  return true;
}

function beginSettling(model) {
  if (model.state !== "incoming") {
    return false;
  }

  model.state = "settling";
  model.settlingPresented = {};
  return true;
}

function settlingFramePresented(model, screenName) {
  var name = String(screenName);
  var count;

  if (model.state !== "settling") {
    return false;
  }
  if (model.expected.indexOf(name) === -1) {
    abort(model);
    return false;
  }

  count = model.settlingPresented[name] || 0;
  if (count >= 2) {
    return false;
  }
  model.settlingPresented[name] = count + 1;

  for (var i = 0; i < model.expected.length; i++) {
    if ((model.settlingPresented[model.expected[i]] || 0) < 2) {
      return true;
    }
  }

  model.state = "done";
  return true;
}

function watchdogTimeoutMs(model) {
  if (model.state === "capturing") {
    return CAPTURE_TIMEOUT_MS;
  }
  if (model.state === "captured") {
    return CAPTURED_TIMEOUT_MS;
  }
  if (model.state === "outgoing" || model.state === "covered"
      || model.state === "incoming" || model.state === "settling") {
    return model.totalDurationMs + CLEANUP_MARGIN_MS;
  }
  return 0;
}

function expireWatchdog(model) {
  if (watchdogTimeoutMs(model) === 0) {
    return false;
  }

  return abort(model);
}
