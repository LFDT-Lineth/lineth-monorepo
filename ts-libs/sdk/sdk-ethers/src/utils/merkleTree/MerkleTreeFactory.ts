import { SparseMerkleTree } from "./SparseMerkleTree";

export class SparseMerkleTreeFactory {
  private readonly depth: number;

  /**
   * Initializes a new instance of the `SparseMerkleTreeFactory`.
   *
   * @param {number} depth - The depth of the sparse Merkle trees to be created. This determines the maximum number of leaves the tree can accommodate.
   */
  constructor(depth: number) {
    this.depth = depth;
  }

  /**
   * Creates a new instance of a `SparseMerkleTree` with the factory's configured depth.
   *
   * @returns {SparseMerkleTree} A new `SparseMerkleTree` instance with the specified depth.
   */
  public create(): SparseMerkleTree {
    return new SparseMerkleTree(this.depth);
  }

  /**
   * Creates a new `SparseMerkleTree` and populates it with the provided leaves.
   *
   * This method initializes a new tree and adds each leaf from the provided array to the tree. The index of each leaf in the array corresponds to its key in the tree.
   *
   * @param {string[]} leaves - An array of leaf values to add to the tree. The index of each leaf in the array is used as its key.
   * @returns {SparseMerkleTree} A `SparseMerkleTree` instance populated with the provided leaves.
   */
  public createAndAddLeaves(leaves: string[]): SparseMerkleTree {
    const tree = new SparseMerkleTree(this.depth);
    for (const [index, leaf] of leaves.entries()) {
      tree.addLeaf(index, leaf);
    }
    return tree;
  }
}
