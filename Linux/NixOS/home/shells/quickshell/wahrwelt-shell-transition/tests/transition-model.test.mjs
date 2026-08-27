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
  assert.equal(model.durationMs, 5000, "the visible reveal must last exactly 5000 ms");
  assert.equal(transitionModel.capture(model, "DP-1"), true);
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(transitionModel.capture(model, "DP-1"), false);
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(transitionModel.capture(model, "HDMI-A-1"), true);
  assert.equal(transitionModel.status(model), "captured");
}

{
  const model = createTwoScreenModel();
  assert.equal(transitionModel.reveal(model), false);
  assert.equal(transitionModel.status(model), "capturing");
  assert.equal(transitionModel.progress(model, 5000), 0);
}

{
  const model = createTwoScreenModel();
  transitionModel.capture(model, "DP-1");
  transitionModel.capture(model, "HDMI-A-1");
  assert.equal(transitionModel.reveal(model), true);
  assert.equal(transitionModel.status(model), "revealing");
  assert.equal(transitionModel.progress(model, -1), 0);
  assert.equal(transitionModel.progress(model, 2500), 0.5);
  assert.equal(transitionModel.progress(model, 4999), 0.9998);
  assert.equal(transitionModel.status(model), "revealing");
  assert.equal(transitionModel.progress(model, 5000), 1);
  assert.equal(
    transitionModel.status(model),
    "revealing",
    "relative progress must not replace NumberAnimation completion",
  );
  assert.equal(transitionModel.completeReveal(model), true);
  assert.equal(transitionModel.status(model), "done");
  assert.equal(transitionModel.completeReveal(model), false);
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
  assert.equal(transitionModel.reveal(model), false);
  assert.equal(transitionModel.progress(model, 5000), 0);
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
  transitionModel.reveal(model);
  assert.equal(transitionModel.status(model), "revealing");
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
  assert.equal(transitionModel.watchdogTimeoutMs(model), 30000);
  assert.equal(transitionModel.expireWatchdog(model), true);
  assert.equal(transitionModel.status(model), "aborted");
}

{
  const model = transitionModel.create(["DP-1"]);
  transitionModel.capture(model, "DP-1");
  assert.equal(transitionModel.reveal(model), true);
  assert.equal(transitionModel.watchdogTimeoutMs(model), 6000);
  assert.equal(transitionModel.expireWatchdog(model), true);
  assert.equal(transitionModel.status(model), "aborted");
}

console.log("OK shell transition state and progress model");
