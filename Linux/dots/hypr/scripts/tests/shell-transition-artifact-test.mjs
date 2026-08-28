#!/usr/bin/env node

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";

const artifactArgument = process.argv[2];
if (
  !artifactArgument
  || !fs.lstatSync(artifactArgument, { throwIfNoEntry: false })?.isDirectory()
) {
  process.stderr.write(`usage: ${process.argv[1]} ARTIFACT\n`);
  process.exit(64);
}
const artifact = path.resolve(artifactArgument);

const expectedFiles = [
  "shell.qml",
  "TransitionController.qml",
  "transition-model.js",
  "shaders/honeycomb.frag",
  "shaders/honeycomb.frag.qsb",
  "shaders/LICENSE-Noctalia-MIT.txt",
];

const artifactFiles = new Map();
const pendingDirectories = [artifact];
while (pendingDirectories.length > 0) {
  const directory = pendingDirectories.pop();
  for (const name of fs.readdirSync(directory)) {
    const absolute = path.join(directory, name);
    const relative = path.relative(artifact, absolute);
    assert.ok(relative !== "" && !relative.startsWith(`..${path.sep}`)
      && !path.isAbsolute(relative), `artifact path escapes root: ${absolute}`);

    const entry = fs.lstatSync(absolute);
    if (entry.isDirectory()) {
      pendingDirectories.push(absolute);
    } else {
      assert.ok(entry.isFile(),
        `artifact entry must be a regular file or directory: ${relative}`);
      artifactFiles.set(relative, absolute);
    }
  }
}

for (const file of expectedFiles) {
  assert.ok(artifactFiles.has(file), `managed transition artifact is missing ${file}`);
}

assert.ok(fs.statSync(path.join(artifact, "shaders/honeycomb.frag.qsb")).size > 0,
  "compiled honeycomb shader must be nonempty");

const modelSource = fs.readFileSync(path.join(artifact, "transition-model.js"), "utf8");
const transitionModel = {};
vm.createContext(transitionModel);
vm.runInContext(modelSource, transitionModel);
for (const [profile, bridgeDurationMs, totalDurationMs] of [
  ["caelestia", 3000, 9000],
  ["noctalia", 3000, 9000],
  ["end4", 5000, 11000],
  ["end4-pc", 5000, 11000],
]) {
  const model = transitionModel.create(["contract-screen"], profile);
  assert.equal(model.outgoingDurationMs, 3000,
    `${profile} old shell cover contract must be exactly 3000 ms`);
  assert.equal(model.bridgeDurationMs, bridgeDurationMs,
    `${profile} opaque handoff contract`);
  assert.equal(model.incomingDurationMs, 3000,
    `${profile} new shell reveal contract must be exactly 3000 ms`);
  assert.equal(model.totalDurationMs, totalDurationMs,
    `${profile} complete visible transition contract`);
}

const shellSource = fs.readFileSync(path.join(artifact, "shell.qml"), "utf8");
assert.match(shellSource, /id:\s*neutralVeilSource/,
  "transition artifact must contain the opaque animated handoff veil");
assert.match(shellSource, /easing\.type: Easing\.InOutCubic/,
  "shell cover and reveal must use a smooth eased curve");

const shaderSource = fs.readFileSync(
  path.join(artifact, "shaders/honeycomb.frag"), "utf8");
assert.match(shaderSource, /frozenAlpha \* globalFade/,
  "honeycomb edges must fade throughout each phase");

const notice = fs.readFileSync(
  path.join(artifact, "shaders/LICENSE-Noctalia-MIT.txt"), "utf8");
assert.match(notice, /^MIT License$/m, "shader artifact must retain the MIT notice");
assert.match(notice, /Permission is hereby granted/, "MIT grant text must be present");

const forbiddenMarker = Buffer.from("WlSessionLock");
for (const [file, absolute] of artifactFiles) {
  assert.equal(fs.readFileSync(absolute).includes(forbiddenMarker), false,
    `${file} must not use WlSessionLock`);
}

console.log(`OK shell transition artifact ${artifact}`);
