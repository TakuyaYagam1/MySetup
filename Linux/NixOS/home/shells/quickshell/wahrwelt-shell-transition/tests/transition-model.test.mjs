import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const testDir = path.dirname(fileURLToPath(import.meta.url));
const modelPath = path.join(testDir, "..", "transition-model.js");

assert.ok(
  fs.existsSync(modelPath),
  "transition-model.js must implement the transition state machine",
);

const source = fs.readFileSync(modelPath, "utf8");
const transitionModel = {};
vm.createContext(transitionModel);
vm.runInContext(source, transitionModel);

function createTwoScreenModel() {
  return transitionModel.create(["DP-1", "HDMI-A-1"]);
}

{
  const model = createTwoScreenModel();
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(model.outgoingDurationMs, 3000, "old shell cover must last exactly 3000 ms");
  assert.equal(model.bridgeDurationMs, 4000, "covered shell handoff must last exactly 4000 ms");
  assert.equal(model.incomingDurationMs, 3000, "new shell reveal must last exactly 3000 ms");
  assert.equal(model.totalDurationMs, 10000, "the complete visible transition must last exactly 10000 ms");
  assert.equal(transitionModel.capture(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(transitionModel.capture(model, "DP-1"), false);
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(transitionModel.capture(model, "HDMI-A-1"), true);
  assert.equal(transitionModel.status(model), "captured");
}

{
  const model = createTwoScreenModel();
  assert.equal(transitionModel.beginTransition(model), false);
  assert.equal(transitionModel.status(model), "capturing");
}

{
  const model = createTwoScreenModel();
  transitionModel.capture(model, "DP-1");
  transitionModel.capture(model, "HDMI-A-1");
  assert.equal(transitionModel.beginTransition(model), true);
  assert.equal(transitionModel.status(model), "outgoing");
  assert.equal(transitionModel.beginTransition(model), false);
  assert.equal(transitionModel.coverFramePresented(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "outgoing");
  assert.equal(transitionModel.coverFramePresented(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "outgoing");
  assert.equal(transitionModel.coverFramePresented(model, "DP-1"), false);
  assert.equal(transitionModel.coverFramePresented(model, "HDMI-A-1"), true);
  assert.equal(transitionModel.status(model), "outgoing");
  assert.equal(transitionModel.coverFramePresented(model, "HDMI-A-1"), true);
  assert.equal(transitionModel.status(model), "covered");
  assert.equal(transitionModel.coverFramePresented(model, "HDMI-A-1"), false);
  assert.equal(transitionModel.beginIncoming(model), true);
  assert.equal(transitionModel.status(model), "incoming");
  assert.equal(transitionModel.beginSettling(model), true);
  assert.equal(transitionModel.status(model), "settling");
  assert.equal(transitionModel.settlingFramePresented(model, "DP-1"), true);
  assert.equal(transitionModel.settlingFramePresented(model, "HDMI-A-1"), true);
  assert.equal(transitionModel.status(model), "settling");
  assert.equal(transitionModel.settlingFramePresented(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "settling");
  assert.equal(transitionModel.settlingFramePresented(model, "HDMI-A-1"), true);
  assert.equal(transitionModel.status(model), "done");
  assert.equal(transitionModel.settlingFramePresented(model, "HDMI-A-1"), false);
}

{
  const model = createTwoScreenModel();
  assert.equal(transitionModel.screensMatch(model, ["HDMI-A-1", "DP-1"]), true);
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(transitionModel.screensMatch(model, ["DP-1"]), false);
  assert.equal(transitionModel.status(model), "aborted");
}

{
  const model = createTwoScreenModel();
  assert.equal(transitionModel.captureFailed(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "aborted");
  assert.equal(transitionModel.capture(model, "HDMI-A-1"), false);
  assert.equal(transitionModel.beginTransition(model), false);
}

{
  const model = transitionModel.create(["DP-1"]);
  transitionModel.capture(model, "DP-1");
  assert.equal(transitionModel.status(model), "captured");
  assert.equal(transitionModel.captureFailed(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "aborted");
}

{
  const model = transitionModel.create(["DP-1"]);
  transitionModel.capture(model, "DP-1");
  transitionModel.beginTransition(model);
  assert.equal(transitionModel.status(model), "outgoing");
  assert.equal(transitionModel.captureFailed(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "aborted");
}

{
  const model = createTwoScreenModel();
  assert.equal(transitionModel.abort(model), true);
  assert.equal(transitionModel.status(model), "aborted");
  assert.equal(transitionModel.abort(model), false);
}

assert.equal(
  typeof transitionModel.expireWatchdog,
  "function",
  "Qt Timer expiry must abort without consulting a wall clock",
);

{
  const model = transitionModel.create(["DP-1"]);
  assert.equal(transitionModel.watchdogTimeoutMs(model), 5000);
  assert.equal(transitionModel.expireWatchdog(model), true);
  assert.equal(transitionModel.status(model), "aborted");
  assert.equal(transitionModel.expireWatchdog(model), false);
}

{
  const model = transitionModel.create(["DP-1"]);
  transitionModel.capture(model, "DP-1");
  assert.equal(transitionModel.status(model), "captured");
  assert.equal(transitionModel.watchdogTimeoutMs(model), 1000);
  assert.equal(transitionModel.expireWatchdog(model), true);
  assert.equal(transitionModel.status(model), "aborted");
}

{
  const model = transitionModel.create(["DP-1"]);
  transitionModel.capture(model, "DP-1");
  assert.equal(transitionModel.beginTransition(model), true);
  assert.equal(transitionModel.watchdogTimeoutMs(model), 10750);
  assert.equal(transitionModel.coverFramePresented(model, "DP-1"), true);
  assert.equal(transitionModel.coverFramePresented(model, "DP-1"), true);
  assert.equal(transitionModel.watchdogTimeoutMs(model), 10750);
  assert.equal(transitionModel.beginIncoming(model), true);
  assert.equal(transitionModel.watchdogTimeoutMs(model), 10750);
  assert.equal(transitionModel.expireWatchdog(model), true);
  assert.equal(transitionModel.status(model), "aborted");
}

console.log("OK shell transition 3 + 4 + 3 state model");
