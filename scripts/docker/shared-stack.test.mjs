import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { cpSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const run = (command, args, env = {}) =>
  execFileSync(command, args, { cwd: root, encoding: "utf8", env: { ...process.env, ...env } });
const compose = (files, env = {}) =>
  JSON.parse(
    run("docker", ["compose", ...files.flatMap((file) => ["-f", file]), "config", "--format", "json"], {
      COMPOSE_PROFILES: "l1,l2,riscv",
      ...env,
    }),
  );

test("RISC-V uses the shared infrastructure and coordinator runtime", () => {
  const tags = {
    LINEA_BESU_PACKAGE_TAG: "test-besu",
    LINEA_COORDINATOR_TAG: "test-coordinator",
    MARU_TAG: "test-maru",
  };
  const base = compose(["docker/compose-tracing-v2.yml"], tags);
  const riscv = compose(["docker/compose-tracing-v2.yml", "docker/compose-riscv.yml"], tags);
  assert.equal(riscv.name, base.name);
  assert.deepEqual(riscv.networks, base.networks);
  for (const service of ["l1-el-node", "l1-cl-node", "l1-node-genesis-generator", "postgres"]) {
    assert.deepEqual(riscv.services[service], base.services[service], service);
  }
  for (const service of ["sequencer", "maru", "coordinator"]) {
    assert.equal(riscv.services[service].image, base.services[service].image);
    assert.notEqual(riscv.services[service].pull_policy, "never");
  }
  assert.deepEqual(riscv.services.coordinator.command, base.services.coordinator.command);
  for (const [service, target] of [
    ["coordinator", "/data"],
    ["sequencer", "/var/lib/besu/log4j.xml"],
  ]) {
    const mount = (model) => model.services[service].volumes.find((volume) => volume.target === target);
    assert.deepEqual(mount(riscv), mount(base));
  }
  assert.ok(riscv.services["riscv-proof-responder"]);
  assert.ok(!riscv.services["prover-v3"]);
});

for (const mode of ["zkevm", "riscv"]) {
  test(`${mode} initializes matching Besu and Maru genesis`, (t) => {
    const directory = mkdtempSync(join(tmpdir(), "local-genesis-"));
    t.after(() => rmSync(directory, { recursive: true, force: true }));
    for (const file of ["genesis-besu.json.template", "genesis-maru.json.template"]) {
      cpSync(join(root, "docker/config/l2-genesis-initialization", file), join(directory, file));
    }
    run("sh", ["docker/config/l2-genesis-initialization/init.sh"], {
      L2_GENESIS_DIRECTORY: directory,
      L2_GENESIS_MODE: mode,
    });
    const besu = JSON.parse(readFileSync(join(directory, "genesis-besu.json"), "utf8"));
    const maru = JSON.parse(readFileSync(join(directory, "genesis-maru.json"), "utf8"));
    assert.equal(besu.config.osakaTime, 0);
    assert.equal(besu.config.amsterdamTime, mode === "riscv" ? 0 : undefined);
    assert.equal(maru.config["0"].elFork, mode === "riscv" ? "Amsterdam" : "Osaka");
    assert.equal(besu.timestamp, readFileSync(join(directory, "fork-timestamp.txt"), "utf8").trim());
  });
}
