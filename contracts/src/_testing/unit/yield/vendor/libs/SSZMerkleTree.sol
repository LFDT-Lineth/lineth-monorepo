// SPDX-License-Identifier: UNLICENSED
// for testing purposes only

pragma solidity 0.8.33;

import { GIndex, pack, concat, fls } from "../../../../../yield/libs/vendor/lido/GIndex.sol";
import { SSZ } from "../../../../../yield/libs/vendor/lido/SSZ.sol";

import { SSZBLSHelpers } from "./SSZBLSHelpers.sol";

/// Merkle tree Implementation that aligns with CL implementation
/// NOT gas optimized, for testing proposes only
contract SSZMerkleTree is SSZBLSHelpers {
  uint256 public immutable TREE_DEPTH; // Adjustable tree depth
  uint256 public leafCount = 0; // Number of leaves in the tree
  mapping(uint256 => bytes32) public nodes; // Merkle tree nodes mapping

  /// @notice Initializes the Merkle tree with a given depth and pre-filled nodes so GIndex can closesly match CL
  constructor(GIndex base) {
    TREE_DEPTH = depth(base);
    // allows to simulate middle part of the tree
    leafCount = base.index() - (1 << TREE_DEPTH);
  }

  /// @notice Adds a new leaf to the tree
  /// @param leaf The leaf value (hashed data)
  /// @return index The index of the added leaf
  function addLeaf(bytes32 leaf) public returns (uint256) {
    require(leafCount < (1 << TREE_DEPTH), "Tree is full");

    uint256 index = (1 << TREE_DEPTH) + leafCount; // Compute SSZ generalized index
    nodes[index] = leaf;
    leafCount++;

    _updateTree(index); // Update the Merkle tree structure

    return index;
  }

  /// @notice Computes the Merkle root of the tree
  /// @return root The computed root hash
  function getMerkleRoot() public view returns (bytes32) {
    return nodes[1]; // The root of the tree
  }

  /// @notice Computes and returns the Merkle proof for a given leaf index
  /// @param leafIndex The index of the leaf in the tree
  /// @return proof The array of proof hashes
  function getMerkleProof(uint256 leafIndex) public view returns (bytes32[] memory) {
    require(leafIndex < leafCount, "Invalid leaf index");

    uint256 index = (1 << TREE_DEPTH) + leafIndex;
    bytes32[] memory proof = new bytes32[](TREE_DEPTH);

    for (uint256 i = 0; i < TREE_DEPTH; i++) {
      uint256 siblingIndex = index % 2 == 0 ? index + 1 : index - 1;
      proof[i] = nodes[siblingIndex];
      index /= 2;
    }
    return proof;
  }

  /// @notice Returns the SSZ generalized index of a given leaf position
  /// @param position The position of the leaf (0-based)
  /// @return generalizedIndex The SSZ generalized index
  function getGeneralizedIndex(uint256 position) public view returns (GIndex) {
    require(position < (1 << TREE_DEPTH), "Invalid position");

    return pack((1 << TREE_DEPTH) + position, uint8(TREE_DEPTH));
  }

  /// @dev Updates the tree after adding a leaf
  /// @param index The index of the new leaf
  function _updateTree(uint256 index) internal {
    while (index > 1) {
      uint256 parentIndex = index / 2;
      uint256 siblingIndex = index % 2 == 0 ? index + 1 : index - 1;

      bytes32 left = nodes[index % 2 == 0 ? index : siblingIndex];
      bytes32 right = nodes[index % 2 == 0 ? siblingIndex : index];

      nodes[parentIndex] = sha256(abi.encodePacked(left, right));

      index = parentIndex;
    }
  }

  function addValidatorLeaf(SSZBLSHelpers.Validator calldata validator) public returns (uint256) {
    return addLeaf(validatorHashTreeRootCalldata(validator));
  }

  /// @notice Modified version of `verify` from Solady `MerkleProofLib` to support generalized indices and sha256 precompile.
  /// @dev Reverts if `leaf` doesn't exist in the Merkle tree with `root`, given `proof`.
  function getRoot(bytes32[] memory proof, bytes32 leaf, GIndex gI) external view returns (bytes32 root) {
    uint256 index = gI.index();

    /// @solidity memory-safe-assembly
    assembly {
      let length := mload(proof)
      // Check if `proof` is empty.
      if iszero(length) {
        // revert InvalidProof()
        mstore(0x00, 0x09bde339)
        revert(0x1c, 0x04)
      }
      // Initialize `offset` to the offset of `proof` in the calldata.
      let offset := add(0x20, proof)
      // Left shift by 5 is equivalent to multiplying by 0x20.
      let end := add(offset, shl(5, length))
      // Iterate over proof elements to compute root hash.
      // prettier-ignore
      for { } 1 { } {
                // Slot of `leaf` in scratch space.
                // If the condition is true: 0x20, otherwise: 0x00.
                let scratch := shl(5, and(index, 1))
                index := shr(1, index)
                if iszero(index) {
                    // revert BranchHasExtraItem()
                    mstore(0x00, 0x5849603f)
                    // 0x1c = 28 => offset in 32-byte word of a slot 0x00
                    revert(0x1c, 0x04)
                }
                // Store elements to hash contiguously in scratch space.
                // Scratch space is 64 bytes (0x00 - 0x3f) and both elements are 32 bytes.
                mstore(scratch, leaf)
                // load next proof from calldata to the scratch space at 0x00 or 0x20
                // xor() acts as if
                mstore(xor(scratch, 0x20), mload(offset))
                // Call sha256 precompile.
                let result := staticcall(
                    gas(),
                    0x02, // SHA-256 precompile
                    0x00, // input from scratch space from 0x00
                    0x40, // length is 2 leafs of 32 bytes each
                    0x00, // output back to scratch space at 0x00
                    0x20  // length of the output is 32 bytes
                )

                if iszero(result) {
                    // Precompile returns no data on OutOfGas error.
                    revert(0, 0)
                }

                // Reuse `leaf` to store the hash to reduce stack operations.
                leaf := mload(0x00)
                offset := add(offset, 0x20)
                if iszero(lt(offset, end)) {
                    break
                }
            }

      if iszero(eq(index, 1)) {
        // revert BranchHasMissingItem()
        mstore(0x00, 0x1b6661c3)
        revert(0x1c, 0x04)
      }

      root := leaf
    }
  }
}
