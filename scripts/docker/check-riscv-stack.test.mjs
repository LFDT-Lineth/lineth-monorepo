import assert from "node:assert/strict";
import test from "node:test";
import { validateSample } from "./check-riscv-stack.mjs";

function fixture() {
  const request = {
    metadata: { startBlockNumber: 4, endBlockNumber: 4 },
    programVk: "0x1234",
    proofRequest: {
      chainConfig: { forkName: "Amsterdam", chainId: 1337 },
      payloads: [
        {
          statelessInput: {
            newPayloadRequest: {
              executionPayload: {
                blockNumber: 4,
                blockHash: "0x5678",
                transactions: ["0xabcd"],
                blockAccessList: "0xc0",
                slotNumber: 4,
              },
            },
            executionWitness: { state: ["0xc0"], codes: [], headers: ["0xc0"] },
          },
        },
      ],
    },
  };
  const response = {
    proverVersion: "riscv-local-dev",
    startBlockNumber: 4,
    publicInputs: { endBlockNumber: 4 },
    programVk: "0x1234",
  };
  const block = { number: "0x4", hash: "0x5678", transactions: ["0x9abc"] };
  return { request, response, block };
}

test("accepts a canonical transaction block with a matching dummy response", () => {
  const { request, response, block } = fixture();
  assert.equal(validateSample(request, response, block), 4);
});

for (const [name, mutate] of [
  [
    "empty block",
    ({ request }) =>
      (request.proofRequest.payloads[0].statelessInput.newPayloadRequest.executionPayload.transactions = []),
  ],
  ["missing witness", ({ request }) => (request.proofRequest.payloads[0].statelessInput.executionWitness.state = [])],
  ["wrong fork", ({ request }) => (request.proofRequest.chainConfig.forkName = "Osaka")],
  [
    "missing Amsterdam slot",
    ({ request }) =>
      delete request.proofRequest.payloads[0].statelessInput.newPayloadRequest.executionPayload.slotNumber,
  ],
  ["stale chain data", ({ block }) => (block.hash = "0xffff")],
  ["wrong response interval", ({ response }) => (response.publicInputs.endBlockNumber = 5)],
  ["wrong response program", ({ response }) => (response.programVk = "0xffff")],
]) {
  test(`rejects ${name}`, () => {
    const data = fixture();
    mutate(data);
    assert.throws(() => validateSample(data.request, data.response, data.block));
  });
}
