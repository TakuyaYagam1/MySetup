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

function createTwoScreenModel(profile = "caelestia") {
  return transitionModel.create(["DP-1", "HDMI-A-1"], profile);
}

{
  const model = createTwoScreenModel();
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(model.outgoingDurationMs, 3000, "old shell cover must last exactly 3000 ms");
  assert.equal(model.bridgeDurationMs, 3000, "normal covered shell handoff must last exactly 3000 ms");
  assert.equal(model.bridgePulseCount, 3, "normal covered shell handoff must show exactly three pulses");
  assert.equal(model.incomingDurationMs, 3000, "new shell reveal must last exactly 3000 ms");
  assert.equal(model.totalDurationMs, 9000, "the complete normal visible transition must last exactly 9000 ms");
  assert.equal(transitionModel.capture(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(transitionModel.capture(model, "DP-1"), false);
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(transitionModel.capture(model, "HDMI-A-1"), true);
  assert.equal(transitionModel.status(model), "captured");
}

for (const profile of ["caelestia", "noctalia"]) {
  const model = transitionModel.create(["DP-1"], profile);
  assert.equal(model.targetProfile, profile);
  assert.equal(model.bridgeDurationMs, 3000, `${profile} must retain the three-second covered handoff`);
  assert.equal(model.bridgePulseCount, 3, `${profile} must retain exactly three covered pulses`);
  assert.equal(model.totalDurationMs, 9000, `${profile} must retain the nine-second visible timeline`);
  assert.equal(transitionModel.capture(model, "DP-1"), true);
  assert.equal(transitionModel.beginTransition(model), true);
  assert.equal(transitionModel.watchdogTimeoutMs(model), 10750,
    `${profile} watchdog must include the cover-fence and cleanup margins`);
}

for (const profile of ["end4", "end4-pc"]) {
  const model = transitionModel.create(["DP-1"], profile);
  assert.equal(model.targetProfile, profile);
  assert.equal(model.bridgeDurationMs, 5000, `${profile} must receive a five-second covered handoff`);
  assert.equal(model.bridgePulseCount, 5, `${profile} must receive exactly five covered pulses`);
  assert.equal(model.totalDurationMs, 11000, `${profile} must receive the eleven-second visible timeline`);
  assert.equal(transitionModel.capture(model, "DP-1"), true);
  assert.equal(transitionModel.beginTransition(model), true);
  assert.equal(transitionModel.watchdogTimeoutMs(model), 12750,
    `${profile} watchdog must include the cover-fence and cleanup margins`);
}

for (const profile of ["", "END4", "end4-pc ", "unknown"]) {
  const model = transitionModel.create(["DP-1"], profile);
  assert.equal(transitionModel.status(model), "aborted",
    `invalid exact target profile ${JSON.stringify(profile)} must fail closed`);
  assert.equal(transitionModel.beginTransition(model), false);
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
  const model = transitionModel.create(["DP-1"], "caelestia");
  transitionModel.capture(model, "DP-1");
  assert.equal(transitionModel.status(model), "captured");
  assert.equal(transitionModel.captureFailed(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "aborted");
}

{
  const model = transitionModel.create(["DP-1"], "caelestia");
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
  const model = transitionModel.create(["DP-1"], "caelestia");
  assert.equal(transitionModel.watchdogTimeoutMs(model), 5000);
  assert.equal(transitionModel.expireWatchdog(model), true);
  assert.equal(transitionModel.status(model), "aborted");
  assert.equal(transitionModel.expireWatchdog(model), false);
}

{
  const model = transitionModel.create(["DP-1"], "caelestia");
  transitionModel.capture(model, "DP-1");
  assert.equal(transitionModel.status(model), "captured");
  assert.equal(transitionModel.watchdogTimeoutMs(model), 1000);
  assert.equal(transitionModel.expireWatchdog(model), true);
  assert.equal(transitionModel.status(model), "aborted");
}

{
  const model = transitionModel.create(["DP-1"], "caelestia");
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

console.log("OK profile-aware shell transition state model");
