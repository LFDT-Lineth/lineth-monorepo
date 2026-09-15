import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { setTimeout } from "node:timers/promises";
import { pathToFileURL } from "node:url";

// Check real execution inputs; the response is deliberately a dummy proof.
export function validateSample(request, response, block) {
  const { startBlockNumber, endBlockNumber } = request.metadata;
  assert(Number.isSafeInteger(startBlockNumber) && startBlockNumber > 0);
  assert.equal(endBlockNumber, startBlockNumber, "Expected one block per execution request");
  assert.equal(request.proofRequest.chainConfig.forkName, "Amsterdam");
  assert.equal(request.proofRequest.chainConfig.chainId, 1337);
  assert.equal(request.proofRequest.payloads.length, 1);
  const { executionPayload: payload } = request.proofRequest.payloads[0].statelessInput.newPayloadRequest;
  const { executionWitness: witness } = request.proofRequest.payloads[0].statelessInput;
  assert.equal(payload.blockNumber, startBlockNumber);
  assert.equal(payload.blockHash, block.hash, "Request must refer to the canonical L2 block");
  assert.equal(BigInt(block.number), BigInt(startBlockNumber));
  assert(payload.transactions.length > 0, "Require a transaction-bearing block, not just empty blocks");
  assert.equal(payload.transactions.length, block.transactions.length);
  assert.match(payload.blockAccessList, /^0x[0-9a-f]+$/i);
  assert(Number.isSafeInteger(payload.slotNumber), "Amsterdam slotNumber is required");
  assert(witness.state.length > 0 && witness.headers.length > 0, "Execution witness must be populated");
  assert.equal(response.startBlockNumber, startBlockNumber);
  assert.equal(response.publicInputs.endBlockNumber, endBlockNumber);
  assert.equal(response.programVk, request.programVk);
  assert.equal(response.proverVersion, "riscv-local-dev");
  return startBlockNumber;
}

async function rpc(port, method, params = []) {
  const response = await fetch(`http://127.0.0.1:${port}`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
    signal: AbortSignal.timeout(10_000),
  });
  assert(response.ok, `RPC HTTP status ${response.status}`);
  const body = await response.json();
  assert(!body.error, `${method}: ${JSON.stringify(body.error)}`);
  assert(body.result != null, `${method} returned no result`);
  return body.result;
}

async function main() {
  const dataDir = process.env.RISCV_DATA_DIR || "tmp/riscv";
  const executionDir = path.join(dataDir, "prover/riscv/execution");
  const reportDir = "tmp/riscv-ci";
  await mkdir(reportDir, { recursive: true });
  assert.equal(await rpc(8445, "eth_chainId"), "0x1e2eaac");
  assert.equal(await rpc(8545, "eth_chainId"), "0x539");
  for (const [port, address] of [
    [8445, "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9"],
    [8545, "0xe537D669CA013d86EBeF1D64e40fC74CADC91987"],
  ]) {
    assert.notEqual(await rpc(port, "eth_getCode", [address, "latest"]), "0x", `No contract at ${address}`);
  }
  const initialHead = BigInt(await rpc(8545, "eth_blockNumber"));
  const deadline = Date.now() + 120_000;
  console.log("Waiting for L2 progress and a completed transaction-bearing execution batch...");
  while (Date.now() < deadline) {
    const files = (await readdir(path.join(executionDir, "requests"))).filter((name) =>
      name.endsWith("-getZkL2ExecutionProof.json"),
    );
    // This stack starts RISC-V at genesis; ensure block 1 was requested too.
    if (files.some((name) => name.startsWith("1-1-")) && BigInt(await rpc(8545, "eth_blockNumber")) > initialHead) {
      for (const file of files) {
        const request = JSON.parse(await readFile(path.join(executionDir, "requests", file), "utf8"));
        if (!request.proofRequest.payloads[0].statelessInput.newPayloadRequest.executionPayload.transactions.length) {
          continue;
        }
        let response;
        try {
          response = JSON.parse(await readFile(path.join(executionDir, "responses", file), "utf8"));
        } catch (error) {
          if (error.code === "ENOENT") continue;
          throw error;
        }
        const number = request.metadata.startBlockNumber;
        const block = await rpc(8545, "eth_getBlockByNumber", [`0x${BigInt(number).toString(16)}`, false]);
        validateSample(request, response, block);
        const persisted = execFileSync(
          "docker",
          [
            "compose",
            "-p",
            process.env.RISCV_COMPOSE_PROJECT || "linea-riscv-dev",
            "-f",
            process.env.RISCV_COMPOSE_FILE || "docker/compose-riscv.yml",
            "exec",
            "-T",
            "postgres",
            "psql",
            "-U",
            "postgres",
            "-d",
            "linea_coordinator",
            "-Atc",
            `select count(*) from batches where start_block_number = ${number} and end_block_number = ${number} and status = 2;`,
          ],
          { encoding: "utf8", timeout: 10_000 },
        ).trim();
        if (Number(persisted) < 1) continue;
        const summary = `PASS: Amsterdam from block 1, L2 progressing, transaction block ${number} has a witness, dummy response and persisted batch.\n`;
        await writeFile(path.join(reportDir, "request.json"), JSON.stringify(request, null, 2));
        await writeFile(path.join(reportDir, "response.json"), JSON.stringify(response, null, 2));
        await writeFile(path.join(reportDir, "result.txt"), summary);
        console.log(summary.trim());
        return;
      }
    }
    await setTimeout(1_000);
  }
  throw new Error("Timed out after 120s waiting for a completed transaction-bearing RISC-V execution batch");
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch(async (error) => {
    console.error(error);
    await writeFile("tmp/riscv-ci/result.txt", `FAIL: ${error.message}\n`).catch(() => {});
    process.exitCode = 1;
  });
}
