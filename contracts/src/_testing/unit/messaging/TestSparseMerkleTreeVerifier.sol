// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { SparseMerkleTreeVerifier } from "../../../messaging/libraries/SparseMerkleTreeVerifier.sol";
import { EfficientLeftRightKeccak } from "../../../libraries/EfficientLeftRightKeccak.sol";

contract TestSparseMerkleTreeVerifier {
  using SparseMerkleTreeVerifier for *;
  using EfficientLeftRightKeccak for *;

  function verifyMerkleProof(
    bytes32 _leafHash,
    bytes32[] calldata _proof,
    uint32 _leafIndex,
    bytes32 _root
  ) external pure returns (bool) {
    return SparseMerkleTreeVerifier._verifyMerkleProof(_leafHash, _proof, _leafIndex, _root);
  }

  function efficientKeccak(bytes32 _left, bytes32 _right) external pure returns (bytes32 value) {
    return EfficientLeftRightKeccak._efficientKeccak(_left, _right);
  }

  function testSafeCastToUint32(uint256 _value) external pure returns (uint32) {
    return SparseMerkleTreeVerifier.safeCastToUint32(_value);
  }

  function getLeafHash(
    address _from,
    address _to,
    uint256 _fee,
    uint256 _value,
    uint256 _messageNumber,
    bytes calldata _calldata
  ) external pure returns (bytes32) {
    return keccak256(abi.encodePacked(_from, _to, _fee, _value, _messageNumber, _calldata));
  }
}
