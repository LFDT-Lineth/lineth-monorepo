import { describe, it } from "@jest/globals";

import { generateHexString } from "../../testing/helpers";
import { SparseMerkleTree } from "../SparseMerkleTree";

describe("TestSparseMerkleTree", () => {
  describe("Initialization", () => {
    it("should throw an error if depth <= 1", () => {
      expect(() => new SparseMerkleTree(1)).toThrow("Merkle tree depth must be greater than 1");
    });

    it("should return initialized tree", () => {
      const tree = new SparseMerkleTree(5);
      expect(tree.getRoot()).toEqual("0x0eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d");
    });
  });

  describe("getLeaf", () => {
    it("should throw an error when leaf index is lower than 0", () => {
      const tree = new SparseMerkleTree(5);
      expect(() => tree.getLeaf(-1)).toThrow("Leaf index is out of range");
    });

    it("should throw an error when leaf index is greater than 2 ** depth", () => {
      const tree = new SparseMerkleTree(5);
      expect(() => tree.getLeaf(2 ** 5 + 1)).toThrow("Leaf index is out of range");
    });

    it("should return leaf", () => {
      const tree = new SparseMerkleTree(5);
      const messageHash = generateHexString(32);
      tree.addLeaf(0, messageHash);

      expect(tree.getLeaf(0)).toEqual(messageHash);
    });
  });

  describe("getRoot", () => {
    it("should return merkle root", () => {
      const tree = new SparseMerkleTree(5);
      const messageHashes = [
        "0x0000000000000000000000000000000000000000000000000000000000000000",
        "0x1111111111111111111111111111111111111111111111111111111111111111",
        "0x2222222222222222222222222222222222222222222222222222222222222222",
        "0x3333333333333333333333333333333333333333333333333333333333333333",
        "0x4444444444444444444444444444444444444444444444444444444444444444",
      ];

      for (let i = 0; i < messageHashes.length; i++) {
        tree.addLeaf(i, messageHashes[i]);
      }

      expect(tree.getRoot()).toEqual("0x97d2505cd0c868c753353628fbb1aacc52bba62ddebac0536256e1e8560d4f27");
    });
  });

  describe("addLeaf", () => {
    it("should add a new leaf to the tree", () => {
      const tree = new SparseMerkleTree(5);
      const messageHashes = [
        "0x0000000000000000000000000000000000000000000000000000000000000000",
        "0x1111111111111111111111111111111111111111111111111111111111111111",
      ];

      for (let i = 0; i < messageHashes.length; i++) {
        tree.addLeaf(i, messageHashes[i]);
      }

      const rootBeforeLeafAddition = tree.getRoot();
      expect(rootBeforeLeafAddition).toEqual("0xa31eac4d98942f1ccfe8d3110babea107f3fee363bd34caa954c686d897ee353");

      tree.addLeaf(2, "0x2222222222222222222222222222222222222222222222222222222222222222");

      expect(tree.getRoot()).not.toEqual(rootBeforeLeafAddition);
    });
  });

  describe("getProof", () => {
    it("should throw an error when leaf index is lower than 0", () => {
      const tree = new SparseMerkleTree(5);
      expect(() => tree.getProof(-1)).toThrow("Leaf index is out of range");
    });

    it("should throw an error when leaf index is greater than 2 ** depth", () => {
      const tree = new SparseMerkleTree(5);
      expect(() => tree.getProof(2 ** 5 + 1)).toThrow("Leaf index is out of range");
    });

    it("should throw an error when leaf value is empty", () => {
      const tree = new SparseMerkleTree(5);
      expect(() => tree.getProof(0)).toThrow("Leaf does not exist");
    });

    it.each([
      {
        leafIndex: 1,
        expectedProof: [
          "0x0000000000000000000000000000000000000000000000000000000000000000",
          "0xf3357627f4934d47fe409005b05c900777a6d97ec3788304e2d9c7b4d322cd4d",
          "0xa6de4f0dc4d6b6915a7e3e8c60f6bdf237f8329edd48041b974dcab7fe5bcfc6",
          "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
          "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
        ],
      },
      {
        leafIndex: 2,
        expectedProof: [
          "0x3333333333333333333333333333333333333333333333333333333333333333",
          "0x8e4b8e18156a1c7271055ce5b7ef53bb370294ebd631a3b95418a92da46e681f",
          "0xa6de4f0dc4d6b6915a7e3e8c60f6bdf237f8329edd48041b974dcab7fe5bcfc6",
          "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
          "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
        ],
      },
      {
        leafIndex: 3,
        expectedProof: [
          "0x2222222222222222222222222222222222222222222222222222222222222222",
          "0x8e4b8e18156a1c7271055ce5b7ef53bb370294ebd631a3b95418a92da46e681f",
          "0xa6de4f0dc4d6b6915a7e3e8c60f6bdf237f8329edd48041b974dcab7fe5bcfc6",
          "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
          "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
        ],
      },
      {
        leafIndex: 4,
        expectedProof: [
          "0x0000000000000000000000000000000000000000000000000000000000000000",
          "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
          "0xd287edfff411d3b45e9c7bf7186d7e9d44fa2a0fe36d85154165da0a1d7ce5bd",
          "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
          "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
        ],
      },
    ])("should return the proof for leaf with index $leafIndex", ({ leafIndex, expectedProof }) => {
      const tree = new SparseMerkleTree(5);
      const messageHashes = [
        "0x0000000000000000000000000000000000000000000000000000000000000000",
        "0x1111111111111111111111111111111111111111111111111111111111111111",
        "0x2222222222222222222222222222222222222222222222222222222222222222",
        "0x3333333333333333333333333333333333333333333333333333333333333333",
        "0x4444444444444444444444444444444444444444444444444444444444444444",
      ];

      for (let i = 0; i < messageHashes.length; i++) {
        tree.addLeaf(i, messageHashes[i]);
      }

      expect(tree.getProof(leafIndex)).toStrictEqual({
        leafIndex,
        proof: expectedProof,
        root: "0x97d2505cd0c868c753353628fbb1aacc52bba62ddebac0536256e1e8560d4f27",
      });
    });
  });
});
