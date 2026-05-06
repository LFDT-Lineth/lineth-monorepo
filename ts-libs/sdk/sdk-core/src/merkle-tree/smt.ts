import { zeroHash } from "../constants/bytes";
import { MessageProof } from "../types/message";
import { Hex } from "../types/misc";

/**
 * A sparse Merkle tree implementation using 1-based flat indexing.
 *
 * Flat-Tree Schema
 * - Root is at index 1.
 * - For any node at index `i`:
 *   - Left child: `2 * i`
 *   - Right child: `2 * i + 1`
 *   - Parent: `Math.floor(i / 2)`
 * - Leaves occupy indices from `2^depth` through `2^(depth+1) - 1`.
 *   - Leaf with zero-based key `k` is at index `2^depth + k`.
 *
 * This scheme allows O(log N) updates and proofs without building the full tree.
 *
 * Flat-Tree Indexing (depth = 3 example):
 *
 *      Level 0              [1]
 *                        /      \
 *                       /        \
 *                      /          \
 *                     /            \
 *      Level 1      [2]             [3]
 *                   /   \           /   \
 *                  /     \         /     \
 *                 /       \       /       \
 *      Level 2   [4]     [5]      [6]      [7]
 *                / \     / \      / \      / \
 *               /   \   /   \    /   \    /   \
 *      Level 3 [8] [9] [10][11] [12][13] [14][15]
 */
export class SparseMerkleTree {
  /** Tree depth (number of levels to leaves) */
  private depth: number;
  /** Hash function to use for hashing nodes */
  private hashFn: (left: Hex, right: Hex) => Hex;
  /** Map storing only non-default node hashes keyed by their flat-tree index */
  private nodeMap = new Map<bigint, Hex>();

  /**
   * Precomputed hashes of empty subtrees at each height:
   * zeroHashes[i] = merkle root of an empty subtree of height `i`.
   * - `zeroHashes[0] = ZERO_HASH`  (empty leaf)
   * - `zeroHashes[1] = H(ZERO_HASH, ZERO_HASH)`
   * - …
   * - `zeroHashes[depth] = root` of a fully-empty tree
   */
  private zeroHashes: Hex[];

  /**
   * @param depth The depth of the tree (must be > 1).
   * @throws If depth <= 1.
   */
  constructor(depth: number, hashFn: (left: Hex, right: Hex) => Hex) {
    if (depth <= 1) {
      throw new Error("Merkle tree depth must be greater than 1");
    }

    this.hashFn = hashFn;
    this.depth = depth;
    this.zeroHashes = this.buildZeroHashes(depth);
    // Seed the root (index 1) to the empty-tree root
    this.nodeMap.set(1n, this.zeroHashes[depth]);
  }

  /**
   * Build the zeroHashes array: empty subtree roots for heights 0..depth.
   * @private
   * @param {number} depth The depth of the tree.
   * @returns {string[]} An array of hashes for empty subtrees at each height.
   */
  private buildZeroHashes(depth: number): Hex[] {
    const z: Hex[] = [zeroHash];
    for (let i = 1; i <= depth; i++) {
      z[i] = this.hash(z[i - 1], z[i - 1]);
    }
    return z;
  }

  /**
   * Get the current Merkle root (hash at index 1).
   *
   * @returns {Hex} The root hash of the Merkle tree.
   */
  public getRoot(): Hex {
    return this.nodeMap.get(1n)!;
  }

  /**
   * Computes the hash of two child node values using the ethers `solidityPackedKeccak256` hashing function.
   *
   * @param {string} left The value of the left child node.
   * @param {string} right The value of the right child node.
   * @returns {string} The hash of the two child node values.
   */
  private hash(left: Hex, right: Hex): Hex {
    return this.hashFn(left, right);
  }

  /**
   * Compute flat-tree index for a leaf.
   * @private
   * @param {number} idx Zero-based leaf index (0 ≤ idx < 2^depth).
   * @throws If idx out of range.
   * @returns {number} The depth of the tree.
   */
  private leafNodeIndex(idx: number): bigint {
    if (idx < 0 || idx >= 1 << this.depth) {
      throw new Error("Leaf index is out of range");
    }
    // Flat-tree leaf index: 2^depth + idx
    return (1n << BigInt(this.depth)) + BigInt(idx);
  }

  /**
   * Compute parent index.
   * @private
   * @param {bigint} nodeIdx The index of the child node.
   * @returns {bigint} The index of the parent node.
   */
  private parentIndex(nodeIdx: bigint): bigint {
    return nodeIdx >> 1n;
  }

  /**
   * Compute sibling index.
   * @private
   * @param {bigint} nodeIdx The index of the child node.
   * @returns {bigint} The index of the sibling node.
   */
  private siblingIndex(nodeIdx: bigint): bigint {
    return (nodeIdx & 1n) === 1n ? nodeIdx - 1n : nodeIdx + 1n;
  }

  /**
   * Fallback hash for missing nodes at given height above leaves.
   * @private
   * @param {number} height The height of the node.
   * @returns {string} The fallback hash for the specified height.
   */
  private fallbackHash(height: number): Hex {
    return this.zeroHashes[height];
  }

  /**
   * Retrieve the hash of a leaf. Returns ZERO_HASH if unset.
   * @param {number} idx Zero-based leaf index (0 ≤ idx < 2^depth).
   * @throws If idx out of range.
   * @returns {string} The value of the leaf at the specified key.
   */
  public getLeaf(idx: number): string {
    const leafIdx = this.leafNodeIndex(idx);
    return this.nodeMap.get(leafIdx) ?? this.fallbackHash(0);
  }

  /**
   * Insert or update a leaf at given index, then rebalance the path to root.
   * @param {number} idx Zero-based leaf index (0 ≤ idx < 2^depth).
   * @param {string} leafHash The hash to set at that leaf.
   * @throws If idx out of range.
   */
  public addLeaf(idx: number, leafHash: Hex): void {
    const nodeIdx = this.leafNodeIndex(idx);
    this.nodeMap.set(nodeIdx, leafHash);
    this.rehashPath(nodeIdx);
  }

  /**
   * Walk from a node up to root, recomputing and storing each parent.
   * @private
   * @param {bigint} startIdx The index of the starting node.
   * @returns {void}
   */
  private rehashPath(startIdx: bigint): void {
    let nodeIdx = startIdx;

    for (let level = this.depth; level > 0; level--) {
      const currentHeight = this.depth - level;

      const siblingIdx = this.siblingIndex(nodeIdx);

      const leftIdx = (nodeIdx & 1n) === 1n ? siblingIdx : nodeIdx;
      const rightIdx = (nodeIdx & 1n) === 1n ? nodeIdx : siblingIdx;

      const fallbackHash = this.fallbackHash(currentHeight);
      const leftHash = this.nodeMap.get(leftIdx) ?? fallbackHash;
      const rightHash = this.nodeMap.get(rightIdx) ?? fallbackHash;

      nodeIdx = this.parentIndex(nodeIdx);

      this.nodeMap.set(nodeIdx, this.hash(leftHash, rightHash));
    }
  }

  /**
   * Generate a Merkle proof for a leaf, in leaf-to-root order.
   * @param {number} idx Zero-based leaf index (0 ≤ idx < 2^depth).
   * @throws If idx out of range or leaf is unset.
   * @returns {Proof} An object containing the proof elements, the root hash, and the leaf index.
   */
  public getProof(idx: number): MessageProof {
    const leafIdx = this.leafNodeIndex(idx);

    const leafHash = this.nodeMap.get(leafIdx) ?? this.fallbackHash(0);

    if (leafHash === this.fallbackHash(0)) {
      throw new Error("Leaf does not exist");
    }

    const proof: Hex[] = [];
    let nodeIdx = leafIdx;

    for (let level = this.depth; level > 0; level--) {
      const currentheight = this.depth - level;
      const siblingIdx = this.siblingIndex(nodeIdx);
      const fallbackHash = this.fallbackHash(currentheight);
      const siblingHash = this.nodeMap.get(siblingIdx) ?? fallbackHash;

      proof.push(siblingHash);

      nodeIdx = this.parentIndex(nodeIdx);
    }

    return {
      proof,
      root: this.getRoot(),
      leafIndex: idx,
    };
  }
}
