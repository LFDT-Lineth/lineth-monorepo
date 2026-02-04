import { WEI_PER_GWEI, SLOTS_PER_EPOCH } from "../../core/constants/blockchain";
import { gweiToWei, weiToGwei, weiToGweiNumber, get0x02WithdrawalCredentials, slotToEpoch } from "../blockchain";

describe("weiToGwei", () => {
  it("should return zero when given zero wei", () => {
    // Arrange
    const zeroWei = 0n;

    // Act
    const result = weiToGwei(zeroWei);

    // Assert
    expect(result).toBe(0n);
  });

  it("should convert exact multiples of WEI_PER_GWEI to whole gwei", () => {
    // Arrange
    const fiveGweiInWei = 5n * WEI_PER_GWEI;

    // Act
    const result = weiToGwei(fiveGweiInWei);

    // Assert
    expect(result).toBe(5n);
  });

  it("should floor fractional gwei values", () => {
    // Arrange
    const wholeGweiPart = 1234n;
    const fractionalWeiPart = 567n;
    const fractionalWei = wholeGweiPart * WEI_PER_GWEI + fractionalWeiPart;

    // Act
    const result = weiToGwei(fractionalWei);

    // Assert
    expect(result).toBe(wholeGweiPart);
  });
});

describe("weiToGweiNumber", () => {
  it("should return a number representation of gwei", () => {
    // Arrange
    const gweiValue = 7n;
    const weiValue = gweiValue * WEI_PER_GWEI;

    // Act
    const result = weiToGweiNumber(weiValue);

    // Assert
    expect(result).toBe(7);
  });

  it("should handle large but safe values", () => {
    // Arrange
    const gweiValue = 9_000_000n;
    const weiValue = gweiValue * WEI_PER_GWEI;

    // Act
    const result = weiToGweiNumber(weiValue);

    // Assert
    expect(result).toBe(Number(gweiValue));
  });
});

describe("gweiToWei", () => {
  it("should convert gwei to wei", () => {
    // Arrange
    const gweiValue = 42n;

    // Act
    const result = gweiToWei(gweiValue);

    // Assert
    expect(result).toBe(42n * WEI_PER_GWEI);
  });

  it("should be the inverse of weiToGwei for whole gwei amounts", () => {
    // Arrange
    const gweiValue = 1_234_567n;

    // Act
    const wei = gweiToWei(gweiValue);
    const convertedBack = weiToGwei(wei);

    // Assert
    expect(convertedBack).toBe(gweiValue);
  });
});

describe("get0x02WithdrawalCredentials", () => {
  it("should convert a valid lowercase address to withdrawal credentials format", () => {
    // Arrange
    const lowercaseAddress = "0x2101af8b812b529fc303c976b6dd747618cfdadb";
    const expectedCredentials = "0x0200000000000000000000002101af8b812b529fc303c976b6dd747618cfdadb";

    // Act
    const result = get0x02WithdrawalCredentials(lowercaseAddress);

    // Assert
    expect(result).toBe(expectedCredentials);
  });

  it("should convert a valid uppercase address to lowercase withdrawal credentials", () => {
    // Arrange
    const uppercaseAddress = "0x2101AF8B812B529FC303C976B6DD747618CFDADB";
    const expectedCredentials = "0x0200000000000000000000002101af8b812b529fc303c976b6dd747618cfdadb";

    // Act
    const result = get0x02WithdrawalCredentials(uppercaseAddress);

    // Assert
    expect(result).toBe(expectedCredentials);
  });

  it("should convert a valid mixed-case address to lowercase withdrawal credentials", () => {
    // Arrange
    const mixedCaseAddress = "0x2101Af8B812b529Fc303c976B6Dd747618cFdAdB";
    const expectedCredentials = "0x0200000000000000000000002101af8b812b529fc303c976b6dd747618cfdadb";

    // Act
    const result = get0x02WithdrawalCredentials(mixedCaseAddress);

    // Assert
    expect(result).toBe(expectedCredentials);
  });

  it("should handle zero address", () => {
    // Arrange
    const zeroAddress = "0x0000000000000000000000000000000000000000";
    const expectedCredentials = "0x0200000000000000000000000000000000000000000000000000000000000000";

    // Act
    const result = get0x02WithdrawalCredentials(zeroAddress);

    // Assert
    expect(result).toBe(expectedCredentials);
  });

  it("should throw error for address that is too short", () => {
    // Arrange
    const tooShortAddress = "0x2101af8b812b529fc303c976b6dd747618cfdad";

    // Act & Assert
    expect(() => get0x02WithdrawalCredentials(tooShortAddress)).toThrow("Invalid Ethereum address");
  });

  it("should throw error for address that is too long", () => {
    // Arrange
    const tooLongAddress = "0x2101af8b812b529fc303c976b6dd747618cfdadba";

    // Act & Assert
    expect(() => get0x02WithdrawalCredentials(tooLongAddress)).toThrow("Invalid Ethereum address");
  });

  it("should throw error for address missing 0x prefix", () => {
    // Arrange
    const addressWithoutPrefix = "2101af8b812b529fc303c976b6dd747618cfdadb";

    // Act & Assert
    expect(() => get0x02WithdrawalCredentials(addressWithoutPrefix)).toThrow("Invalid Ethereum address");
  });

  it("should throw error for address with invalid hex characters", () => {
    // Arrange
    const addressWithInvalidHex = "0x2101af8b812b529fc303c976b6dd747618cfdadg";

    // Act & Assert
    expect(() => get0x02WithdrawalCredentials(addressWithInvalidHex)).toThrow("Invalid Ethereum address");
  });

  it("should throw error for empty string", () => {
    // Arrange
    const emptyString = "";

    // Act & Assert
    expect(() => get0x02WithdrawalCredentials(emptyString)).toThrow("Invalid Ethereum address");
  });
});

describe("slotToEpoch", () => {
  it("should return zero when given zero slot", () => {
    // Arrange
    const zeroSlot = 0;

    // Act
    const result = slotToEpoch(zeroSlot);

    // Assert
    expect(result).toBe(0);
  });

  it("should convert one full epoch of slots to epoch 1", () => {
    // Arrange
    const oneEpochInSlots = 1 * SLOTS_PER_EPOCH;

    // Act
    const result = slotToEpoch(oneEpochInSlots);

    // Assert
    expect(result).toBe(1);
  });

  it("should convert five full epochs of slots to epoch 5", () => {
    // Arrange
    const fiveEpochsInSlots = 5 * SLOTS_PER_EPOCH;

    // Act
    const result = slotToEpoch(fiveEpochsInSlots);

    // Assert
    expect(result).toBe(5);
  });

  it("should convert one hundred full epochs of slots to epoch 100", () => {
    // Arrange
    const oneHundredEpochsInSlots = 100 * SLOTS_PER_EPOCH;

    // Act
    const result = slotToEpoch(oneHundredEpochsInSlots);

    // Assert
    expect(result).toBe(100);
  });

  it("should floor slot 31 to epoch 0", () => {
    // Arrange
    const lastSlotOfEpochZero = 31;

    // Act
    const result = slotToEpoch(lastSlotOfEpochZero);

    // Assert
    expect(result).toBe(0);
  });

  it("should floor slot 33 to epoch 1", () => {
    // Arrange
    const secondSlotOfEpochOne = 33;

    // Act
    const result = slotToEpoch(secondSlotOfEpochOne);

    // Assert
    expect(result).toBe(1);
  });

  it("should floor slot 63 to epoch 1", () => {
    // Arrange
    const lastSlotOfEpochOne = 63;

    // Act
    const result = slotToEpoch(lastSlotOfEpochOne);

    // Assert
    expect(result).toBe(1);
  });

  it("should convert slot 64 to epoch 2", () => {
    // Arrange
    const firstSlotOfEpochTwo = 64;

    // Act
    const result = slotToEpoch(firstSlotOfEpochTwo);

    // Assert
    expect(result).toBe(2);
  });

  it("should convert first slot of epoch 1 correctly", () => {
    // Arrange
    const firstSlotOfEpochOne = 32;

    // Act
    const result = slotToEpoch(firstSlotOfEpochOne);

    // Assert
    expect(result).toBe(1);
  });

  it("should convert last slot of epoch 0 correctly", () => {
    // Arrange
    const lastSlotOfEpochZero = 31;

    // Act
    const result = slotToEpoch(lastSlotOfEpochZero);

    // Assert
    expect(result).toBe(0);
  });

  it("should convert first slot of epoch 2 correctly", () => {
    // Arrange
    const firstSlotOfEpochTwo = 64;

    // Act
    const result = slotToEpoch(firstSlotOfEpochTwo);

    // Assert
    expect(result).toBe(2);
  });

  it("should convert last slot of epoch 1 correctly", () => {
    // Arrange
    const lastSlotOfEpochOne = 63;

    // Act
    const result = slotToEpoch(lastSlotOfEpochOne);

    // Assert
    expect(result).toBe(1);
  });

  it("should handle exact large slot number for epoch 1000", () => {
    // Arrange
    const targetEpoch = 1000;
    const slot = targetEpoch * SLOTS_PER_EPOCH;

    // Act
    const result = slotToEpoch(slot);

    // Assert
    expect(result).toBe(targetEpoch);
  });

  it("should floor large slot number with offset of 15 slots", () => {
    // Arrange
    const targetEpoch = 1000;
    const baseSlot = targetEpoch * SLOTS_PER_EPOCH;
    const slotWithOffset = baseSlot + 15;

    // Act
    const result = slotToEpoch(slotWithOffset);

    // Assert
    expect(result).toBe(targetEpoch);
  });

  it("should floor last slot of large epoch 1000", () => {
    // Arrange
    const targetEpoch = 1000;
    const baseSlot = targetEpoch * SLOTS_PER_EPOCH;
    const lastSlotOfEpoch = baseSlot + 31;

    // Act
    const result = slotToEpoch(lastSlotOfEpoch);

    // Assert
    expect(result).toBe(targetEpoch);
  });

  it("should convert first slot of large epoch 1001 correctly", () => {
    // Arrange
    const targetEpoch = 1000;
    const baseSlot = targetEpoch * SLOTS_PER_EPOCH;
    const firstSlotOfNextEpoch = baseSlot + 32;

    // Act
    const result = slotToEpoch(firstSlotOfNextEpoch);

    // Assert
    expect(result).toBe(targetEpoch + 1);
  });
});
