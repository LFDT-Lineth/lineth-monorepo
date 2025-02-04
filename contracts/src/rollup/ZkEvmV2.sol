// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.26;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { L1MessageServiceV1 } from "../messaging/l1/v1/L1MessageServiceV1.sol";
import { IZkEvmV2 } from "./interfaces/IZkEvmV2.sol";
import { IPlonkVerifier } from "../verifiers/interfaces/IPlonkVerifier.sol";
/**
 * @title Contract to manage cross-chain L1 rollup proving.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract ZkEvmV2 is AccessControlUpgradeable, L1MessageServiceV1, IZkEvmV2 {
  uint256 internal constant MODULO_R = 21888242871839275222246405745257275088548364400416034343698204186575808495617;
  bytes32 public constant OPERATOR_ROLE = keccak256("OPERATOR_ROLE");

  /// @dev DEPRECATED in favor of currentFinalizedState hash.
  uint256 public currentTimestamp;

  /// @notice The most recent finalized L2 block number.
  uint256 public currentL2BlockNumber;

  /// @notice The most recent L2 state root hash mapped by block number.
  mapping(uint256 blockNumber => bytes32 stateRootHash) public stateRootHashes;

  /// @notice The verifier address to use for a proof type when proving.
  mapping(uint256 proofType => address verifierAddress) public verifiers;

  /// @dev Total contract storage is 54 slots with the gap below.
  /// @dev Keep 50 free storage slots for future implementation updates to avoid storage collision.
  uint256[50] private __gap;

  /**
   * @notice Verifies the proof with locally computed public inputs.
   * @dev If the verifier based on proof type is not found, it reverts with InvalidProofType.
   * @param _publicInput The computed public input hash cast as uint256.
   * @param _proofType The proof type to determine which verifier contract to use.
   * @param _proof The proof to be verified with the proof type verifier contract.
   */
  function _verifyProof(uint256 _publicInput, uint256 _proofType, bytes calldata _proof) internal {
    uint256[] memory publicInput = new uint256[](1);
    publicInput[0] = _publicInput;

    address verifierToUse = verifiers[_proofType];

    if (verifierToUse == address(0)) {
      revert InvalidProofType();
    }

    (bool callSuccess, bytes memory result) = verifierToUse.call(
      abi.encodeCall(IPlonkVerifier.Verify, (_proof, publicInput))
    );

    if (!callSuccess) {
      if (result.length > 0) {
        assembly {
          let dataOffset := add(result, 0x20)

          // Store the modified first 32 bytes back into memory overwriting the location after having swapped out the selector.
          mstore(
            dataOffset,
            or(
              // InvalidProofOrProofVerificationRanOutOfGas(string) = 0xca389c44bf373a5a506ab5a7d8a53cb0ea12ba7c5872fd2bc4a0e31614c00a85.
              shl(224, 0xca389c44),
              and(mload(dataOffset), 0x00000000ffffffffffffffffffffffffffffffffffffffffffffffffffffffff)
            )
          )

          revert(dataOffset, mload(result))
        }
      } else {
        revert InvalidProofOrProofVerificationRanOutOfGas("Unknown");
      }
    }

    bool proofSucceeded = abi.decode(result, (bool));
    if (!proofSucceeded) {
      revert InvalidProof();
    }
  }
}
