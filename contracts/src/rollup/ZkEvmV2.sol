// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { L1MessageServiceBase } from "../messaging/l1/L1MessageServiceBase.sol";
import { IZkEvmV2 } from "./interfaces/IZkEvmV2.sol";

/**
 * @title Contract to manage cross-chain L1 rollup proving.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract ZkEvmV2 is AccessControlUpgradeable, L1MessageServiceBase, IZkEvmV2 {
  uint256 internal constant MODULO_R = 21888242871839275222246405745257275088548364400416034343698204186575808495617;
  bytes32 public constant OPERATOR_ROLE = keccak256("OPERATOR_ROLE");

  /// @dev DEPRECATED in favor of currentFinalizedState hash.
  uint256 private currentTimestamp_DEPRECATED;

  /// @notice The most recent finalized L2 block number.
  uint256 public currentL2BlockNumber;

  /// @notice The most recent L2 state root hash mapped by block number.
  mapping(uint256 blockNumber => bytes32 stateRootHash) public stateRootHashes;

  /// @notice The verifier address to use for a proof type when proving.
  mapping(uint256 proofType => address verifierAddress) public verifiers;

  /// @dev Total contract storage is 54 slots with the gap below.
  /// @dev Keep 50 free storage slots for future implementation updates to avoid storage collision.
  uint256[50] private __gap;
}
