import { setErrorConfig } from "viem";
import { TEST_MERKLE_ROOT, TEST_MESSAGE_HASH, TEST_TRANSACTION_HASH } from "../../tests/constants";
import {
  EventNotFoundInFinalizationDataError,
  L2BlockNotFinalizedError,
  MerkleRootNotFoundInFinalizationDataError,
  MessageNotFoundError,
  MessagesNotFoundInBlockRangeError,
  MissingMessageProofOrClientForClaimingOnL1Error,
} from "./bridge";

describe("Bridge Errors", () => {
  beforeAll(() => {
    setErrorConfig({ version: "viem@x.y.z" });
  });

  it("MessageNotFoundError", () => {
    expect(
      new MessageNotFoundError({
        hash: TEST_MESSAGE_HASH,
      }),
    ).toMatchInlineSnapshot(`
    [MessageNotFoundError: Message with hash ${TEST_MESSAGE_HASH} not found.

    Version: viem@x.y.z]
  `);
  });

  it("L2BlockNotFinalizedError", () => {
    const blockNumber = 1_000_000n;
    expect(new L2BlockNotFinalizedError({ blockNumber })).toMatchInlineSnapshot(`
    [L2BlockNotFinalizedError: L2 block number ${blockNumber.toString()} is not finalized on L1 yet.

    Version: viem@x.y.z]
  `);
  });

  it("MessagesNotFoundInBlockRangeError", () => {
    const startBlock = 1_000_000n;
    const endBlock = 1_000_100n;
    expect(new MessagesNotFoundInBlockRangeError({ startBlock, endBlock })).toMatchInlineSnapshot(`
    [MessagesNotFoundInBlockRangeError: No messages found in the specified block range on L2.
    Block range: ${startBlock.toString()} - ${endBlock.toString()}
    
    Version: viem@x.y.z]
  `);
  });

  it("MerkleRootNotFoundInFinalizationDataError", () => {
    const merkleRoot = TEST_MERKLE_ROOT;
    const startBlock = 1_000_000n;
    const endBlock = 1_000_100n;

    expect(new MerkleRootNotFoundInFinalizationDataError({ merkleRoot, startBlock, endBlock })).toMatchInlineSnapshot(`
    [MerkleRootNotFoundInFinalizationDataError: Merkle root ${merkleRoot} not found in finalization data.
    Block range: ${startBlock.toString()} - ${endBlock.toString()}
    
    Version: viem@x.y.z]
  `);
  });

  it("EventNotFoundInFinalizationDataError", () => {
    const transactionHash = TEST_TRANSACTION_HASH;
    const eventName = "TestEvent";
    expect(new EventNotFoundInFinalizationDataError({ transactionHash, eventName })).toMatchInlineSnapshot(`
    [EventNotFoundInFinalizationDataError: Event ${eventName} not found in finalization data.
    Transaction hash: ${transactionHash}

    Version: viem@x.y.z]
  `);
  });

  it("MissingMessageProofOrClientForClaimingOnL1Error", () => {
    expect(new MissingMessageProofOrClientForClaimingOnL1Error()).toMatchInlineSnapshot(`
    [MissingMessageProofOrClientForClaimingOnL1Error: Either \`messageProof\` or \`l2Client\` must be provided to claim a message on L1.

    Version: viem@x.y.z]
  `);
  });
});
