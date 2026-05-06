import { BaseError, Hash } from "viem";

export type MessageNotFoundErrorType = MessageNotFoundError & {
  name: "MessageNotFoundError";
};

export class MessageNotFoundError extends BaseError {
  constructor({ hash }: { hash: Hash }) {
    super([`Message with hash ${hash} not found.`].join("\n"), { name: "MessageNotFoundError" });
  }
}

export type L2BlockNotFinalizedErrorType = L2BlockNotFinalizedError & {
  name: "L2BlockNotFinalizedError";
};

export class L2BlockNotFinalizedError extends BaseError {
  constructor({ blockNumber }: { blockNumber: bigint }) {
    super([`L2 block number ${blockNumber.toString()} is not finalized on L1 yet.`].join("\n"), {
      name: "L2BlockNotFinalizedError",
    });
  }
}

export type MessagesNotFoundInBlockRangeErrorType = MessagesNotFoundInBlockRangeError & {
  name: "MessagesNotFoundInBlockRangeError";
};

export class MessagesNotFoundInBlockRangeError extends BaseError {
  constructor({ startBlock, endBlock }: { startBlock: bigint; endBlock: bigint }) {
    super(
      [
        `No messages found in the specified block range on L2.`,
        `Block range: ${startBlock.toString()} - ${endBlock.toString()}`,
      ].join("\n"),
      {
        name: "MessagesNotFoundInBlockRangeError",
      },
    );
  }
}

export type MerkleRootNotFoundInFinalizationDataErrorType = MerkleRootNotFoundInFinalizationDataError & {
  name: "MerkleRootNotFoundInFinalizationDataError";
};

export class MerkleRootNotFoundInFinalizationDataError extends BaseError {
  constructor({ merkleRoot, startBlock, endBlock }: { merkleRoot: Hash; startBlock: bigint; endBlock: bigint }) {
    super(
      [
        `Merkle root ${merkleRoot} not found in finalization data.`,
        `Block range: ${startBlock.toString()} - ${endBlock.toString()}`,
      ].join("\n"),
      {
        name: "MerkleRootNotFoundInFinalizationDataError",
      },
    );
  }
}

export type EventNotFoundInFinalizationDataErrorType = EventNotFoundInFinalizationDataError & {
  name: "EventNotFoundInFinalizationDataError";
};

export class EventNotFoundInFinalizationDataError extends BaseError {
  constructor({ transactionHash, eventName }: { transactionHash: Hash; eventName: string }) {
    super([`Event ${eventName} not found in finalization data.`, `Transaction hash: ${transactionHash}`].join("\n"), {
      name: "EventNotFoundInFinalizationDataError",
    });
  }
}

export type MissingMessageProofOrClientForClaimingOnL1ErrorType = MissingMessageProofOrClientForClaimingOnL1Error & {
  name: "MissingMessageProofOrClientForClaimingOnL1Error";
};

export class MissingMessageProofOrClientForClaimingOnL1Error extends BaseError {
  constructor() {
    super(["Either `messageProof` or `l2Client` must be provided to claim a message on L1."].join("\n"), {
      name: "MissingMessageProofOrClientForClaimingOnL1Error",
    });
  }
}
