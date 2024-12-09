import { ethers } from "ethers";
import { BaseError } from "../../core/errors";
import { ZERO_HASH } from "../../core/constants";
import { Proof } from "../../core/clients/ethereum";

class MerkleTreeNode {
  public value: string;
  public left: MerkleTreeNode | null;
  public right: MerkleTreeNode | null;

  constructor(value: string, left: MerkleTreeNode | null = null, right: MerkleTreeNode | null = null) {
    this.value = value;
    this.left = left;
    this.right = right;
  }
}

export class SparseMerkleTree {
  private root: MerkleTreeNode;
  private readonly depth: number;
  private readonly emptyLeaves: string[];

  /**
   * Constructs a `SparseMerkleTree` instance with a specified depth.
   *
   * @param {number} depth - The depth of the Merkle tree. Must be greater than 1.
   */
  constructor(depth: number) {
    if (depth <= 1) {
      throw new BaseError("Merkle tree depth must be greater than 1");
    }
    this.depth = depth;
    this.emptyLeaves = this.generateEmptyLeaves(this.depth);
    this.root = this.createDefaultNode(this.depth);
  }

  /**
   * Adds a leaf to the Merkle tree at the specified key with the given value.
   *
   * @param {number} key - The key at which to add the leaf. Must be within the range of the tree's depth.
   * @param {string} value - The value of the leaf to add.
   */
  public addLeaf(key: number, value: string): void {
    const binaryKey = this.keyToBinary(key);
    this.root = this.insert(this.root, binaryKey, value, 0);
  }

  /**
   * Generates a proof for the existence of a leaf at the specified key.
   *
   * @param {number} key - The key of the leaf to generate a proof for.
   * @returns {Proof} An object containing the proof elements, the root hash, and the leaf index.
   */
  public getProof(key: number): Proof {
    if (key < 0 || key >= Math.pow(2, this.depth)) {
      throw new BaseError(`Leaf index is out of range`);
    }

    const binaryKey = this.keyToBinary(key);
    const leaf = this.getLeaf(key);

    if (leaf === this.emptyLeaves[0]) {
      throw new BaseError(`Leaf does not exist`);
    }

    return {
      proof: this.createProof(this.root, binaryKey, 0).reverse(),
      root: this.root.value,
      leafIndex: key,
    };
  }

  /**
   * Retrieves the value of a leaf at the specified key.
   *
   * @param {number} key - The key of the leaf to retrieve.
   * @returns {string} The value of the leaf at the specified key.
   */
  public getLeaf(key: number): string {
    if (key < 0 || key >= Math.pow(2, this.depth)) {
      throw new BaseError("Leaf index is out of range");
    }
    const binaryKey = this.keyToBinary(key);
    return this.getLeafHelper(this.root, binaryKey, 0);
  }

  /**
   * Retrieves the root hash of the Merkle tree.
   *
   * @returns {string} The root hash of the Merkle tree.
   */
  public getRoot(): string {
    return this.root.value;
  }

  /**
   * Converts a numerical key into its binary string representation, padded to match the tree's depth.
   *
   * @param {number} key - The key to convert.
   * @returns {string} The binary string representation of the key, padded to the tree's depth.
   */
  private keyToBinary(key: number): string {
    return key.toString(2).padStart(this.depth, "0");
  }

  /**
   * Recursively retrieves the value of a leaf node based on its binary key representation.
   *
   * @param {MerkleTreeNode} node - The current node being traversed.
   * @param {string} binaryKey - The binary key representation of the leaf to retrieve.
   * @param {number} depth - The current depth in the tree.
   * @returns {string} The value of the leaf node.
   */
  private getLeafHelper(node: MerkleTreeNode, binaryKey: string, depth: number): string {
    if (depth === this.depth) {
      return node.value;
    }

    const newDepth = this.depth - depth - 1;
    const newNode = new MerkleTreeNode(this.emptyLeaves[newDepth]);

    if (binaryKey[depth] === "0") {
      return this.getLeafHelper(node.left || newNode, binaryKey, depth + 1);
    }

    return this.getLeafHelper(node.right || newNode, binaryKey, depth + 1);
  }

  /**
   * Inserts a new leaf node into the tree at the specified key, or updates an existing leaf's value.
   *
   * @param {MerkleTreeNode} node - The current node being traversed.
   * @param {string} key - The binary key representation of where to insert the new leaf.
   * @param {string} value - The value to store in the new leaf node.
   * @param {number} depth - The current depth in the tree.
   * @returns {MerkleTreeNode} The new or updated node after insertion.
   */
  private insert(node: MerkleTreeNode, key: string, value: string, depth: number): MerkleTreeNode {
    if (depth === this.depth) {
      return new MerkleTreeNode(value);
    }

    const newDepth = this.depth - depth - 1;
    const defaultNode = this.createDefaultNode(newDepth);

    let newLeft = node.left;
    let newRight = node.right;

    if (key[depth] === "0") {
      newLeft = this.insert(node.left || defaultNode, key, value, depth + 1);
    } else {
      newRight = this.insert(node.right || defaultNode, key, value, depth + 1);
    }

    return new MerkleTreeNode(
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion, @typescript-eslint/no-non-null-asserted-optional-chain
      this.hash(newLeft?.value!, newRight?.value!),
      newLeft,
      newRight,
    );
  }

  /**
   * Generates a proof of existence for a leaf node based on its binary key representation.
   *
   * @param {MerkleTreeNode} node - The current node being traversed.
   * @param {string} key - The binary key representation of the leaf to prove.
   * @param {number} depth - The current depth in the tree.
   * @returns {string[]} An array of hashes representing the proof of existence for the leaf.
   */
  private createProof(node: MerkleTreeNode, key: string, depth: number): string[] {
    if (depth === this.depth) {
      return [];
    }

    const newDepth = this.depth - depth - 1;
    const defaultNode = this.createDefaultNode(newDepth);

    if (key[depth] === "0") {
      const nextNode = node.left || defaultNode;
      const value = node.right ? node.right.value : this.emptyLeaves[newDepth];
      return [value].concat(this.createProof(nextNode, key, depth + 1));
    }

    const nextNode = node.right || defaultNode;
    const value = node.left ? node.left.value : this.emptyLeaves[newDepth];
    return [value].concat(this.createProof(nextNode, key, depth + 1));
  }

  /**
   * Computes the hash of two child node values using the ethers `solidityPackedKeccak256` hashing function.
   *
   * @param {string} left - The value of the left child node.
   * @param {string} right - The value of the right child node.
   * @returns {string} The hash of the two child node values.
   */
  private hash(left: string, right: string): string {
    return ethers.solidityPackedKeccak256(["bytes32", "bytes32"], [left, right]);
  }

  /**
   * Generates an array of hashes representing the empty leaves of the tree, used for initializing the tree structure.
   *
   * @param {number} depth - The depth of the tree.
   * @returns {string[]} An array of hashes representing the empty leaves.
   */
  private generateEmptyLeaves(depth: number): string[] {
    const emptyLeaves = [ZERO_HASH];

    for (let i = 1; i < depth; i++) {
      emptyLeaves.push(this.hash(emptyLeaves[i - 1], emptyLeaves[i - 1]));
    }

    return emptyLeaves;
  }

  /**
   * Creates a default node at a specified depth, initializing its value and children based on the tree's empty leaves.
   *
   * @param {number} depth - The depth at which to create the default node.
   * @returns {MerkleTreeNode} The newly created default node.
   */
  private createDefaultNode(depth: number): MerkleTreeNode {
    if (depth === 0) {
      return new MerkleTreeNode(this.emptyLeaves[0]);
    }

    const child = this.createDefaultNode(depth - 1);
    return new MerkleTreeNode(this.hash(this.emptyLeaves[depth - 1], this.emptyLeaves[depth - 1]), child, child);
  }
}
