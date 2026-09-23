import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const read = (path) => readFileSync(path, "utf8");
function run(command, args, options = {}) {
  return spawnSync(command, args, { encoding: "utf8", cwd: root, ...options });
}
function success(result) {
  assert.equal(result.status, 0, result.stdout + result.stderr);
  return result.stdout;
}
function temporary(t) {
  const directory = mkdtempSync(join(tmpdir(), "shared-stack-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  return directory;
}
function compose(files, profiles) {
  return JSON.parse(
    success(
      run("docker", ["compose", ...files.flatMap((file) => ["-f", file]), "config", "--format", "json"], {
        env: { ...process.env, COMPOSE_PROFILES: profiles },
      }),
    ),
  );
}

test("RISC-V overlays the same infrastructure and persistent state as zkEVM", () => {
  const base = compose(["docker/compose-tracing-v2.yml"], "l1,l2");
  const riscv = compose(
    ["docker/compose-tracing-v2.yml", "docker/compose-riscv.yml", "docker/compose-riscv-from-genesis.yml"],
    "l1,l2,riscv",
  );
  assert.equal(base.name, riscv.name);
  assert.deepEqual(base.networks, riscv.networks);
  for (const [name, volume] of Object.entries(riscv.volumes)) {
    assert.deepEqual(base.volumes[name], volume);
  }
  for (const service of ["l1-el-node", "l1-cl-node", "l1-node-genesis-generator", "postgres"]) {
    assert.deepEqual(base.services[service], riscv.services[service]);
  }
  for (const service of ["l1-el-node", "l1-cl-node", "sequencer", "maru", "postgres"]) {
    const volume = base.services[service].volumes.find(
      (mount) =>
        mount.source ===
        `${service === "sequencer" || service === "maru" || service === "postgres" ? service : service.replace("-node", "")}-data`,
    );
    assert.ok(volume, `${service} must persist its database`);
    assert.ok(riscv.services[service].volumes.some((mount) => JSON.stringify(mount) === JSON.stringify(volume)));
  }
  for (const service of ["sequencer", "maru", "coordinator"]) {
    assert.equal(base.services[service].container_name, riscv.services[service].container_name);
    assert.deepEqual(base.services[service].networks, riscv.services[service].networks);
    assert.deepEqual(base.services[service].ports, riscv.services[service].ports);
  }
  assert.deepEqual(base.services.coordinator.command, riscv.services.coordinator.command);
  assert.match(riscv.services.coordinator.environment.COORDINATOR_CONFIG_FILES, /coordinator-config-riscv.toml$/);
  assert.ok(!riscv.services.coordinator.depends_on.shomei);
  const data = (model) => model.services.coordinator.volumes.find((mount) => mount.target === "/data");
  assert.deepEqual(data(base), data(riscv));
  assert.ok(data(riscv).source.endsWith("/tmp/local"));
  assert.ok(base.services["prover-v3"]);
  assert.ok(base.services.shomei);
  assert.ok(!riscv.services["prover-v3"]);
  assert.ok(!riscv.services.shomei);
  const both = compose(
    ["docker/compose-tracing-v2.yml", "docker/compose-riscv.yml", "docker/compose-riscv-from-genesis.yml"],
    "l1,l2,riscv,zkevm",
  );
  assert.ok(both.services["prover-v3"]);
  assert.ok(both.services["riscv-proof-responder"]);
});

test("the reusable RISC-V layer preserves the zkEVM chain and both proof pipelines", () => {
  const files = ["docker/compose-tracing-v2.yml", "docker/compose-riscv.yml"];
  const base = compose(files.slice(0, 1), "l1,l2");
  const shared = compose(files, "l1,l2");
  for (const service of [
    "l1-el-node",
    "l1-cl-node",
    "l1-node-genesis-generator",
    "l2-genesis-initialization",
    "sequencer",
    "maru",
    "postgres",
    "prover-v3",
    "shomei",
    "l2-node-besu",
  ]) {
    assert.deepEqual(shared.services[service], base.services[service], service);
  }
  assert.deepEqual(shared.services.coordinator.depends_on, base.services.coordinator.depends_on);
  assert.equal(shared.services.coordinator.image, base.services.coordinator.image);
  assert.equal(
    shared.services.coordinator.environment.COORDINATOR_CONFIG_FILES,
    "config/coordinator-config.toml config/coordinator-config-riscv-prover.toml",
  );
  assert.ok(!shared.services["riscv-proof-responder"], "dummy proofs must be opt-in");
  const proverConfig = read(join(root, "docker/config/coordinator/coordinator-config-riscv-prover.toml"));
  assert.doesNotMatch(proverConfig, /disabled|contract-address|starting-block-timestamp|timestamp-based-hard-forks/);
  const both = compose(files, "l1,l2,riscv");
  assert.ok(both.services["prover-v3"]);
  assert.ok(both.services["riscv-proof-responder"]);
});

for (const extension of ["ci-extension", "ci-fleet-extension", "staterecovery-extension", "extra-extension"]) {
  test(`shared persistence remains valid in the ${extension} stack`, () => {
    const model = compose([`docker/compose-tracing-v2-${extension}.yml`], "l1,l2,staterecovery");
    for (const service of Object.values(model.services)) {
      for (const mount of service.volumes ?? []) {
        if (mount.type === "volume") assert.ok(model.volumes[mount.source], mount.source);
      }
    }
  });
}

for (const mode of ["zkevm", "riscv"]) {
  test(`${mode} initialization preserves genesis on restart and rejects a mode change`, (t) => {
    const directory = temporary(t);
    for (const name of ["genesis-besu.json.template", "genesis-maru.json.template"]) {
      cpSync(join(root, "docker/config/l2-genesis-initialization", name), join(directory, name));
    }
    const initialize = (selectedMode) =>
      run("sh", ["docker/config/l2-genesis-initialization/init.sh"], {
        env: { ...process.env, L2_GENESIS_DIRECTORY: directory, L2_GENESIS_MODE: selectedMode },
      });
    success(initialize(mode));
    const files = ["genesis-besu.json", "genesis-maru.json", "fork-timestamp.txt"];
    const snapshot = () => files.map((file) => read(join(directory, file)));
    const before = snapshot();
    const besu = JSON.parse(before[0]);
    const maru = JSON.parse(before[1]);
    assert.equal(besu.config.amsterdamTime, mode === "riscv" ? 0 : undefined);
    assert.equal(maru.config["0"].elFork, mode === "riscv" ? "Amsterdam" : "Osaka");
    assert.equal(besu.timestamp, before[2].trim());
    success(initialize(mode));
    assert.deepEqual(snapshot(), before);
    assert.notEqual(initialize(mode === "zkevm" ? "riscv" : "zkevm").status, 0);
    assert.deepEqual(snapshot(), before);
    if (mode === "zkevm") {
      // A future fork schedule is not an Amsterdam-at-genesis mode switch.
      besu.config.amsterdamTime = Number(before[2].trim()) + 3600;
      writeFileSync(join(directory, "genesis-besu.json"), JSON.stringify(besu));
      success(initialize(mode));
      assert.equal(
        JSON.parse(read(join(directory, "genesis-besu.json"))).config.amsterdamTime,
        besu.config.amsterdamTime,
      );
      writeFileSync(join(directory, "genesis-besu.json"), before[0]);
    }
    rmSync(join(directory, "fork-timestamp.txt"));
    assert.notEqual(initialize(mode).status, 0);
    assert.equal(read(join(directory, "genesis-besu.json")), before[0]);
    assert.ok(!existsSync(join(directory, "fork-timestamp.txt")));
  });
}

function lifecycle(t) {
  const directory = temporary(t);
  for (const folder of ["contracts", "scripts/docker", "docker/config/linea-besu-sequencer", "bin"]) {
    mkdirSync(join(directory, folder), { recursive: true });
  }
  cpSync(join(root, "Makefile"), join(directory, "Makefile"));
  writeFileSync(
    join(directory, "contracts/makefile-contracts.mk"),
    'clean-smc-folders:\n\t@echo clean-contracts >> actions\ndeploy-contracts:\n\t@echo deploy-contracts >> actions\n\t@test "$(FAIL_DEPLOY)" != true\n',
  );
  writeFileSync(
    join(directory, "scripts/docker/images.mk"),
    'docker-build-riscv-besu docker-build-maru docker-build-coordinator:\n\t@echo build-$@ >> actions\n\t@test "$(FAIL_BUILD)" != true\n',
  );
  writeFileSync(join(directory, "docker/config/linea-besu-sequencer/deny-list.txt"), "preserved-deny-list\n");
  writeFileSync(
    join(directory, "bin/docker"),
    `#!/bin/sh
printf '%s\\n' "$*" >> actions
case "$*" in
  *version*) echo linux/amd64 ;;
  *inspect*) echo healthy ;;
  *"ps -q"*) echo container-id ;;
esac
if [ -n "\${FAIL_DOCKER_MATCH:-}" ]; then
  case "$*" in *"$FAIL_DOCKER_MATCH"*) exit 1 ;; esac
fi
`,
    { mode: 0o755 },
  );
  return {
    directory,
    actions: () => (existsSync(join(directory, "actions")) ? read(join(directory, "actions")) : ""),
    start: (args = [], env = {}) =>
      run("make", ["start-env-with-riscv", ...args], {
        cwd: directory,
        env: { ...process.env, PATH: `${join(directory, "bin")}:${process.env.PATH}`, ...env },
      }),
  };
}

test("fresh RISC-V startup builds before shared cleanup, deploys after nodes, then starts coordinator", (t) => {
  const fixture = lifecycle(t);
  success(fixture.start());
  const actions = fixture.actions();
  const build = actions.indexOf("build-docker-build-coordinator");
  const clean = actions.indexOf("down --volumes --remove-orphans");
  const nodes = actions.indexOf("up -d --wait --wait-timeout 600 l1-cl-node maru postgres");
  const deploy = actions.indexOf("deploy-contracts");
  const coordinator = actions.indexOf("up -d --no-deps --wait --wait-timeout 600 riscv-proof-responder coordinator");
  assert.ok(build >= 0 && build < clean && clean < nodes && nodes < deploy && deploy < coordinator, actions);
  assert.match(actions, /-f docker\/compose-tracing-v2.yml -f docker\/compose-riscv.yml/);
  assert.doesNotMatch(actions, /tmp\/riscv|linea-riscv-dev|linea-riscv-local-dev/);
});

test("reuse never cleans or redeploys contracts and keeps existing artifacts", (t) => {
  const fixture = lifecycle(t);
  mkdirSync(join(fixture.directory, "tmp/local"), { recursive: true });
  writeFileSync(join(fixture.directory, "tmp/local/existing-proof"), "proof");
  success(fixture.start(["CLEAN_PREVIOUS_ENV=false", "SKIP_CONTRACTS_DEPLOYMENT=true"]));
  assert.doesNotMatch(fixture.actions(), /clean-contracts|deploy-contracts|down|volume rm|find \/data/);
  assert.equal(read(join(fixture.directory, "tmp/local/existing-proof")), "proof");
  assert.equal(
    read(join(fixture.directory, "docker/config/linea-besu-sequencer/deny-list.txt")),
    "preserved-deny-list\n",
  );
});

test("reuse with contract redeployment is rejected before changing the environment", (t) => {
  const fixture = lifecycle(t);
  assert.notEqual(fixture.start(["CLEAN_PREVIOUS_ENV=false"]).status, 0);
  assert.doesNotMatch(fixture.actions(), /compose|deploy-contracts|volume rm/);
});

for (const failure of ["down --volumes", "l2-genesis-initialization", "up -d --wait", "deployment"]) {
  test(`${failure} failure prevents coordinator startup`, (t) => {
    const fixture = lifecycle(t);
    const result = fixture.start(failure === "deployment" ? ["FAIL_DEPLOY=true"] : [], {
      FAIL_DOCKER_MATCH: failure === "deployment" ? "" : failure,
    });
    assert.notEqual(result.status, 0);
    assert.doesNotMatch(fixture.actions(), /up -d --no-deps/);
    if (failure !== "deployment") assert.doesNotMatch(fixture.actions(), /deploy-contracts/);
  });
}

test("failed image build leaves the environment untouched", (t) => {
  const fixture = lifecycle(t);
  assert.notEqual(fixture.start(["FAIL_BUILD=true"]).status, 0);
  assert.match(fixture.actions(), /build-docker-build-riscv-besu/);
  assert.doesNotMatch(fixture.actions(), /compose|clean-contracts|deploy-contracts|volume rm/);
});

test("failed genesis generation does not publish partial files", (t) => {
  const directory = temporary(t);
  cpSync(
    join(root, "docker/config/l2-genesis-initialization/genesis-besu.json.template"),
    join(directory, "genesis-besu.json.template"),
  );
  const result = run("sh", ["docker/config/l2-genesis-initialization/init.sh"], {
    env: { ...process.env, L2_GENESIS_DIRECTORY: directory, L2_GENESIS_MODE: "riscv" },
  });
  assert.notEqual(result.status, 0);
  for (const file of ["genesis-besu.json", "genesis-maru.json", "fork-timestamp.txt"]) {
    assert.ok(!existsSync(join(directory, file)), file);
  }
});
