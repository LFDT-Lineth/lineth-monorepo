// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { Eip1559RlpEncoder } from "./Eip1559RlpEncoder.sol";

contract TestEip1559RlpEncoder {
  uint256 chainId;

  constructor(uint256 _chainId) {
    chainId = _chainId;
  }

  function encodeEip1559Transaction(
    Eip1559RlpEncoder.Eip1559Transaction calldata _transaction
  ) external view returns (bytes memory rlpEncodedTransaction, bytes32 transactionHash) {
    (rlpEncodedTransaction, transactionHash) = Eip1559RlpEncoder._encodeEip1559Transaction(chainId, _transaction);
  }
}
