import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFile, readdir } from "node:fs/promises";
import { setTimeout } from "node:timers/promises";
import { test } from "node:test";

async function rpc(port, method, params = []) {
  const response = await fetch(`http://localhost:${port}`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
    signal: AbortSignal.timeout(5000),
  });
  assert.ok(response.ok);
  const body = await response.json();
  assert.ok(!body.error, JSON.stringify(body.error));
  return body.result;
}

test("RISC-V produces a new execution request and accepts its dummy proof", { timeout: 200_000 }, async () => {
  assert.ok(BigInt(await rpc(8445, "eth_blockNumber")) > 0n, "L1 must produce blocks");
  const initialL2Block = BigInt(await rpc(8545, "eth_blockNumber"));
  assert.ok(initialL2Block > 0n, "L2 must produce blocks");
  const health = await fetch("http://localhost:9545/health", { signal: AbortSignal.timeout(5000) });
  assert.ok(health.ok, "Coordinator must be healthy");
  assert.equal((await health.json()).status, "UP");

  const directory = "tmp/local/prover/riscv/execution";
  const deadline = Date.now() + 180_000;
  while (Date.now() < deadline) {
    for (const name of await readdir(`${directory}/responses`)) {
      const match = /^(\d+)-(\d+)-([a-f0-9]{64})-getZkL2ExecutionProof\.json$/.exec(name);
      // Requiring a block created after this test started excludes stale proofs.
      if (!match || BigInt(match[1]) <= initialL2Block) continue;
      const request = JSON.parse(await readFile(`${directory}/requests/${name}`, "utf8"));
      const response = JSON.parse(await readFile(`${directory}/responses/${name}`, "utf8"));
      assert.equal(request.proofRequest.chainConfig.forkName, "Amsterdam");
      assert.ok(request.proofRequest.payloads.length > 0);
      for (const payload of request.proofRequest.payloads) {
        assert.ok(Object.keys(payload.statelessInput.executionWitness).length > 0, "Execution witness is required");
        assert.ok(payload.statelessInput.newPayloadRequest);
      }
      assert.equal(BigInt(request.metadata.startBlockNumber), BigInt(match[1]));
      assert.equal(BigInt(request.metadata.endBlockNumber), BigInt(match[2]));
      assert.equal(BigInt(response.startBlockNumber), BigInt(match[1]));
      assert.equal(BigInt(response.publicInputs.endBlockNumber), BigInt(match[2]));
      assert.equal(response.programVk, request.programVk);
      assert.equal(response.proof, "0x00");
      // A batch is persisted only after the coordinator accepts the response.
      const accepted = execFileSync(
        "docker",
        [
          "exec",
          "postgres",
          "sh",
          "-c",
          'exec psql -U "$POSTGRES_USER" -d linea_coordinator -Atc "$1"',
          "sh",
          `SELECT EXISTS (SELECT 1 FROM batches WHERE start_block_number = ${match[1]} AND end_block_number = ${match[2]} AND proof_index_hash = '0x${match[3]}')`,
        ],
        { encoding: "utf8", timeout: 5000 },
      ).trim();
      if (accepted === "t") {
        console.log(`Coordinator accepted RISC-V dummy proof for blocks ${match[1]}..${match[2]}`);
        return;
      }
    }
    await setTimeout(1000);
  }
  assert.fail(`No new RISC-V request/response was accepted after L2 block ${initialL2Block} within 180 seconds`);
});
