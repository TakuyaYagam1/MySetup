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
const shaderSource = fs.readFileSync(
  path.join(sourceDir, "shaders", "honeycomb.frag"),
  "utf8",
);

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
  /function beginTransition\(\)[\s\S]*controller\.beginTransition\(\)[\s\S]*outgoingAnimation\.restart\(\)/,
  "the profile-aware visible timeline must start from the explicit IPC handoff",
);
for (const progressProperty of ["outgoingProgress", "bridgeProgress", "incomingProgress"]) {
  assert.match(
    shellSource,
    new RegExp(`property real ${progressProperty}:\\s*0\\.0`),
    `the renderer must expose ${progressProperty} for the profile-aware timeline`,
  );
}
assert.match(
  shellSource,
  /readonly property string targetProfile:\s*Quickshell\.env\("WAHRWELT_SHELL_TRANSITION_TARGET_PROFILE"\)\s*\|\|\s*""/,
  "the renderer must read the exact destination profile from its scoped environment",
);
assert.match(
  shellSource,
  /Component\.onCompleted:[\s\S]*controller\.initialize\(screenNames\(screens\), targetProfile\)/,
  "the exact destination profile must be validated while the transition model is created",
);
assert.match(
  shellSource,
  /TransitionController\s*\{[\s\S]*onCoveredReady:\s*root\.beginCoveredBridge\(\)/,
  "the covered frame fence must be the only trigger for the visible bridge timer",
);
assert.match(
  shellSource,
  /id:\s*outgoingAnimation[\s\S]*onFinished:\s*root\.enterCoveredBridge\(\)/,
  "the outgoing animation must arm the cover fence without starting the bridge timer",
);
assert.match(
  shellSource,
  /function beginCoveredBridge\(\)[\s\S]*transitionState !== "covered"[\s\S]*bridgeAnimation\.restart\(\)/,
  "the full covered hold must start only after the exact covered state",
);
assert.match(
  shellSource,
  /id:\s*bridgeAnimation[\s\S]*onFinished:\s*root\.enterIncoming\(\)/,
  "incoming reveal must wait for the complete profile-specific covered hold",
);
assert.doesNotMatch(
  shellSource,
  /SequentialAnimation/,
  "one autonomous sequence must not consume covered hold time before the frame fence",
);
assert.match(
  shellSource,
  /readonly property int bridgePulseCount:\s*controller\.transitionModel\s*\?\s*controller\.transitionModel\.bridgePulseCount\s*:\s*0/,
  "the renderer must not dereference its transition model before initialization",
);
assert.match(
  shellSource,
  /bridgeProgress\s*\*\s*Math\.PI\s*\*\s*2\.0\s*\*\s*root\.bridgePulseCount/,
  "the neutral veil must show one pulse per requested covered second",
);
assert.match(
  shellSource,
  /id:\s*neutralVeilSource/,
  "the renderer must keep an opaque animated veil between the two shells",
);
assert.match(
  shellSource,
  /root\.coverFenceArmed[\s\S]*controller\.coverFramePresented\(window\.modelData\.name\)[\s\S]*root\.transitionState === "outgoing"[\s\S]*frameProbe\.Window\.window\.update\(\)/,
  "the cover fence must drain a queued frame and request a second per-output swap",
);
assert.match(
  shellSource,
  /id:\s*doneExitTimer[\s\S]*interval:\s*2000[\s\S]*onTriggered:\s*Qt\.quit\(\)/,
  "done must remain observable for exact shell-side completion before bounded self-cleanup",
);
assert.match(
  shellSource,
  /visible:\s*root\.surfaceVisible/,
  "the transparent settling surface must remain mapped until frame drain completes",
);
assert.match(
  shaderSource,
  /float globalFade = 1\.0 - clamp\(ubuf\.progress, 0\.0, 1\.0\);[\s\S]*frozenAlpha \* globalFade/,
  "display edges must fade throughout each phase instead of snapping at its end",
);
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
