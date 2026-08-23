import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const testDir = path.dirname(fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(testDir, "..", "selector-model.js"), "utf8");
const selectorModel = {};
vm.createContext(selectorModel);
vm.runInContext(source, selectorModel);

const profiles = [
  { id: "noctalia", family: "noctalia", title: "noctalia" },
  { id: "caelestia", family: "caelestia", title: "caelestia-shell" },
  {
    id: "end4",
    family: "end4",
    title: "end-4",
    quickshellConfig: "ii",
    variantLabel: "Official",
  },
  {
    id: "end4-pc",
    family: "end4",
    title: "end-4",
    quickshellConfig: "end4-pC",
    variantLabel: "pC",
  },
];

const cards = selectorModel.buildCards(profiles);
assert.equal(cards.length, 3, "four profiles must render as three family cards");
assert.deepEqual(
  Array.from(cards, card => card.family),
  ["noctalia", "caelestia", "end4"],
);
assert.deepEqual(
  Array.from(cards[2].variants, variant => variant.id),
  ["end4", "end4-pc"],
);
assert.deepEqual(
  Array.from(cards[2].variants, variant => variant.variantLabel),
  ["Official", "pC"],
);

assert.equal(selectorModel.activeFamily(profiles, "end4-pc"), "end4");
assert.equal(selectorModel.end4Target("noctalia", "end4-pc"), "end4-pc");
assert.equal(selectorModel.end4Target("end4", "end4-pc"), "end4");
assert.equal(selectorModel.end4Target("end4-pc", "end4"), "end4-pc");
assert.equal(selectorModel.end4Target("noctalia", "invalid"), "end4");
assert.equal(selectorModel.cardTarget(cards[2], "noctalia", "end4-pc"), "end4-pc");
assert.equal(selectorModel.cardTarget(cards[0], "end4-pc", "end4-pc"), "noctalia");

console.log("OK selector family model");
