import { GetBlockExtraDataParameters, GetBlockExtraDataReturnType } from "../actions/getBlockExtraData";
import { GetL1ToL2MessageStatusParameters, GetL1ToL2MessageStatusReturnType } from "../actions/getL1ToL2MessageStatus";
import { GetL2ToL1MessageStatusParameters, GetL2ToL1MessageStatusReturnType } from "../actions/getL2ToL1MessageStatus";
import {
  GetMessageByMessageHashParameters,
  GetMessageByMessageHashReturnType,
} from "../actions/getMessageByMessageHash";
import { GetMessageProofParameters, GetMessageProofReturnType } from "../actions/getMessageProof";
import {
  GetMessagesByTransactionHashParameters,
  GetMessagesByTransactionHashReturnType,
} from "../actions/getMessagesByTransactionsHash";
import {
  GetTransactionReceiptByMessageHashParameters,
  GetTransactionReceiptByMessageHashReturnType,
} from "../actions/getTransactionReceiptByMessageHash";
import { BlockTag } from "../block";

export interface PublicClient {
  getMessageByMessageHash<T = bigint>(
    args: GetMessageByMessageHashParameters,
  ): Promise<GetMessageByMessageHashReturnType<T>>;
  getMessagesByTransactionHash<T = bigint>(
    args: GetMessagesByTransactionHashParameters,
  ): Promise<GetMessagesByTransactionHashReturnType<T>>;
  getTransactionReceiptByMessageHash<T = bigint>(
    args: GetTransactionReceiptByMessageHashParameters,
  ): Promise<GetTransactionReceiptByMessageHashReturnType<T>>;
}

export interface L1PublicClient extends PublicClient {
  getL2ToL1MessageStatus<T>(args: GetL2ToL1MessageStatusParameters<T>): Promise<GetL2ToL1MessageStatusReturnType>;
  getMessageProof<T>(parameters: GetMessageProofParameters<T>): Promise<GetMessageProofReturnType>;
}

export interface L2PublicClient extends PublicClient {
  getBlockExtraData<blockTag extends BlockTag>(
    args: GetBlockExtraDataParameters<blockTag>,
  ): Promise<GetBlockExtraDataReturnType>;
  getL1ToL2MessageStatus(args: GetL1ToL2MessageStatusParameters): Promise<GetL1ToL2MessageStatusReturnType>;
}
