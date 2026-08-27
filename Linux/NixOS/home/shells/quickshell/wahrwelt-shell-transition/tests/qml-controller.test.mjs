import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const testDir = path.dirname(fileURLToPath(import.meta.url));
const sourceDir = path.join(testDir, "..");
const productionSources = [
  "transition-model.js",
  "TransitionController.qml",
  "shell.qml",
].map((name) => fs.readFileSync(path.join(sourceDir, name), "utf8"));

for (const source of productionSources) {
  assert.doesNotMatch(
    source,
    /Date\.now|phaseStartedAt|revealStartedAt|watchdogExpired/,
    "production timeout and progress paths must not depend on wall-clock timestamps",
  );
}

const shellSource = productionSources[2];
assert.match(
  shellSource,
  /property bool deliveredFrame:\s*false/,
  "each screencopy delegate must remember whether it delivered a frame",
);
assert.match(
  shellSource,
  /else if \(!hasContent && window\.deliveredFrame\)\s*\{\s*root\.captureFailed\(window\.modelData\.name\);/,
  "loss after a delivered screencopy frame must fail open",
);

const executable = process.env.QMLTESTRUNNER || "qmltestrunner";
const result = spawnSync(executable, ["-input", testDir], {
  encoding: "utf8",
  env: {
    ...process.env,
    QT_QPA_PLATFORM: "offscreen",
  },
});

assert.equal(
  result.status,
  0,
  [
    "the pinned Qt runtime must execute the transition controller contract",
    result.stdout,
    result.stderr,
    result.error ? String(result.error) : "",
  ].filter(Boolean).join("\n"),
);

process.stdout.write(result.stdout);
