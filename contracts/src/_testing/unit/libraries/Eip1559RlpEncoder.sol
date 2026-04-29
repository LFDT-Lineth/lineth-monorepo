// SPDX-License-Identifier: Apache-2.0

/**
 * @title Library for RLP Encoding EIP-1559 transactions.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
pragma solidity ^0.8.33;

import { RlpEncoder } from "./RlpEncoder.sol";

/**
 * @title Library for RLP Encoding a type 2 EIP-1559 transactions.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
library Eip1559RlpEncoder {
  /**
   * @notice Supporting data for encoding an EIP-1559 transaction.
   * @dev NB: ChainId is not on the struct to allow flexibility by the consuming contract.
   * @dev NB: Access lists is assumed empty and does not appear here.
   * @dev nonce The sender's nonce.
   * @dev maxPriorityFeePerGas The max priority fee the user will pay.
   * @dev maxFeePerGas The max fee per gas the user will pay.
   * @dev gasLimit The limit of gas that the user is prepared to spend.
   * @dev input The calldata input for the transaction.
   * @dev accessList The access list used.
   * @dev yParity The yParity in the signature.
   * @dev r The r in the signature.
   * @dev s The s in the signature.
   */
  struct Eip1559Transaction {
    uint256 nonce;
    uint256 maxPriorityFeePerGas;
    uint256 maxFeePerGas;
    uint256 gasLimit;
    address to;
    uint256 value;
    bytes input;
    RlpEncoder.AccessList[] accessList;
    uint8 yParity;
    uint256 r;
    uint256 s;
  }

  /**
   * @notice Internal function that encodes bytes correctly with length data.
   * @param _chainId The chainId to encode with.
   * @param _transaction The EIP-1559 transaction excluding chainId.
   * @return rlpEncodedTransaction The RLP encoded transaction for submitting.
   * @return transactionHash The expected transaction hash.
   */
  function _encodeEip1559Transaction(
    uint256 _chainId,
    Eip1559Transaction memory _transaction
  ) internal pure returns (bytes memory rlpEncodedTransaction, bytes32 transactionHash) {
    bytes[] memory fields = new bytes[](12);

    fields[0] = RlpEncoder._encodeUint(_chainId);
    fields[1] = RlpEncoder._encodeUint(_transaction.nonce);
    fields[2] = RlpEncoder._encodeUint(_transaction.maxPriorityFeePerGas);
    fields[3] = RlpEncoder._encodeUint(_transaction.maxFeePerGas);
    fields[4] = RlpEncoder._encodeUint(_transaction.gasLimit);
    fields[5] = RlpEncoder._encodeAddress(_transaction.to);
    fields[6] = RlpEncoder._encodeUint(_transaction.value);
    fields[7] = RlpEncoder._encodeBytes(_transaction.input);
    fields[8] = RlpEncoder._encodeAccessList(_transaction.accessList);
    fields[9] = RlpEncoder._encodeUint(_transaction.yParity);
    fields[10] = RlpEncoder._encodeUint(_transaction.r);
    fields[11] = RlpEncoder._encodeUint(_transaction.s);

    rlpEncodedTransaction = abi.encodePacked(hex"02", RlpEncoder._encodeList(fields));
    transactionHash = keccak256(rlpEncodedTransaction);
  }
}
