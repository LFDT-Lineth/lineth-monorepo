import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";

import { TestRlpEncoder } from "../../../typechain-types";
import { deployFromFactory } from "../common/deployment";

describe("RlpEncoder Library", () => {
  let contract: TestRlpEncoder;

  async function deployTestEip1559RlpEncoderFixture() {
    return deployFromFactory("TestRlpEncoder");
  }
  beforeEach(async () => {
    contract = (await loadFixture(deployTestEip1559RlpEncoderFixture)) as TestRlpEncoder;
  });

  describe("RLP Encoding", () => {
    describe("Bool Encoding", () => {
      it("Encodes false correctly", async () => {
        const encoded = await contract.encodeBool(false);
        expect(encoded).equal("0x80");
      });

      it("Encodes true correctly", async () => {
        const encoded = await contract.encodeBool(true);
        expect(encoded).equal("0x01");
      });
    });

    describe("String Encoding", () => {
      it("Encodes blank string correctly", async () => {
        const encoded = await contract.encodeString("");
        expect(encoded).equal("0x80");
      });

      it("Encodes a short string correctly", async () => {
        const encoded = await contract.encodeString("short");
        const expected = "0x8573686f7274";
        expect(encoded).equal(expected);
      });

      it("Encodes a long string correctly", async () => {
        const encoded = await contract.encodeString(
          "This is a string that is quite long and needs some different encoding",
        );
        const expected =
          "0xb84554686973206973206120737472696e672074686174206973207175697465206c6f6e6720616e64206e6565647320736f6d6520646966666572656e7420656e636f64696e67";
        expect(encoded).equal(expected);
      });
    });
  });

  describe("Int Encoding", () => {
    it("Encodes a negative int correctly", async () => {
      const encoded = await contract.encodeInt(-123456789n);
      expect(encoded).equal("0xa0fffffffffffffffffffffffffffffffffffffffffffffffffffffffff8a432eb");
    });
    it("Encodes a positive int correctly", async () => {
      const encoded = await contract.encodeInt(123456789n);
      expect(encoded).equal("0x84075bcd15");
    });

    // TODO random fuzz type tests
    it("Encodes a very large positive int correctly", async () => {
      const encoded =
        await contract.encodeInt(1234567891234567567891234567789123456789123456789123456789123456789123456789n);
      expect(encoded).equal("0xa002babd9c27f528a06ee127601e68ddbe8c982496f253de820f6f70b684045f15");
    });

    it("Encodes a large positive int correctly", async () => {
      const encoded = await contract.encodeInt(123456789123456756789123456723456789123456789n);
      expect(encoded).equal("0x93058936e53d139a968065bc45c0d9d0540c5f15");
    });

    it("Encodes 0 correctly", async () => {
      const encoded = await contract.encodeInt(0n);
      expect(encoded).equal("0x80");
    });

    it("Encodes 1 correctly", async () => {
      const encoded = await contract.encodeInt(1n);
      expect(encoded).equal("0x01");
    });

    it("Encodes 255 correctly", async () => {
      const encoded = await contract.encodeInt(255n);
      expect(encoded).equal("0x81ff");
    });
  });
});
