/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.utils;

import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Quantity;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.ethereum.rlp.BytesValueRLPOutput;
import org.hyperledger.besu.ethereum.rlp.RLPOutput;

/**
 * Utility methods for encoding transactions for the compressor.
 *
 * <p>The compressor expects transactions in a specific format: sender address (20 bytes) followed
 * by the RLP-encoded transaction for signing (without the signature). This format allows the
 * compressor to avoid expensive signature recovery operations.
 */
public final class TxEncodingUtils {

  private TxEncodingUtils() {}

  /**
   * Returns the sender address as a 20-byte array.
   *
   * @param transaction the transaction (interface)
   * @return sender address bytes
   */
  private static byte[] getSenderBytes(final Transaction transaction) {
    return transaction.getSender().toArray();
  }

  /**
   * Encodes a transaction for signing (without the signature). This matches the Go
   * EncodeTxForSigning function and produces the RLP encoding that would be hashed and signed.
   *
   * <p>This method works with the Transaction interface. For transaction types that require access
   * list or code delegation list, the transaction must be an instance of
   * org.hyperledger.besu.ethereum.core.Transaction.
   *
   * @param transaction the transaction to encode (interface)
   * @return the RLP-encoded transaction for signing
   */
  public static byte[] encodeForSigning(final Transaction transaction) {
    final BytesValueRLPOutput rlpOutput = new BytesValueRLPOutput();
    final TransactionType type = transaction.getType();

    switch (type) {
      case FRONTIER -> encodeLegacyForSigning(rlpOutput, transaction);
      case ACCESS_LIST -> encodeAccessListForSigning(rlpOutput, transaction);
      case EIP1559 -> encodeEip1559ForSigning(rlpOutput, transaction);
      case DELEGATE_CODE -> encodeDelegateCodeForSigning(rlpOutput, transaction);
      default -> throw new IllegalArgumentException("Unsupported transaction type: " + type);
    }

    return rlpOutput.encoded().toArray();
  }

  /**
   * Concatenates sender address and RLP-encoded transaction for signing into a single byte array.
   * This is the format expected by the compressor's stateless compression methods.
   *
   * @param transaction the transaction (interface)
   * @return concatenated from + rlpForSigning
   */
  public static byte[] encodeForCompressor(final Transaction transaction) {
    final byte[] from = getSenderBytes(transaction);
    final byte[] rlpForSigning = encodeForSigning(transaction);
    final byte[] combined = new byte[from.length + rlpForSigning.length];
    System.arraycopy(from, 0, combined, 0, from.length);
    System.arraycopy(rlpForSigning, 0, combined, from.length, rlpForSigning.length);
    return combined;
  }

  private static void encodeLegacyForSigning(
      final BytesValueRLPOutput rlpOutput, final Transaction transaction) {
    rlpOutput.startList();
    rlpOutput.writeLongScalar(transaction.getNonce());
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getGasPrice().orElseThrow()));
    rlpOutput.writeLongScalar(transaction.getGasLimit());
    rlpOutput.writeBytes(getTo(transaction));
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getValue()));
    rlpOutput.writeBytes(transaction.getPayload());
    if (transaction.getChainId().isPresent()) {
      // EIP-155 protected
      rlpOutput.writeBigIntegerScalar(transaction.getChainId().get());
      rlpOutput.writeIntScalar(0);
      rlpOutput.writeIntScalar(0);
    }
    rlpOutput.endList();
  }

  private static void encodeAccessListForSigning(
      final BytesValueRLPOutput rlpOutput, final Transaction transaction) {
    rlpOutput.writeRaw(Bytes.of(transaction.getType().getSerializedType()));
    rlpOutput.startList();
    rlpOutput.writeBigIntegerScalar(transaction.getChainId().orElseThrow());
    rlpOutput.writeLongScalar(transaction.getNonce());
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getGasPrice().orElseThrow()));
    rlpOutput.writeLongScalar(transaction.getGasLimit());
    rlpOutput.writeBytes(getTo(transaction));
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getValue()));
    rlpOutput.writeBytes(transaction.getPayload());
    writeAccessList(rlpOutput, transaction);
    rlpOutput.endList();
  }

  private static void encodeEip1559ForSigning(
      final BytesValueRLPOutput rlpOutput, final Transaction transaction) {
    rlpOutput.writeRaw(Bytes.of(transaction.getType().getSerializedType()));
    rlpOutput.startList();
    rlpOutput.writeBigIntegerScalar(transaction.getChainId().orElseThrow());
    rlpOutput.writeLongScalar(transaction.getNonce());
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getMaxPriorityFeePerGas().orElseThrow()));
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getMaxFeePerGas().orElseThrow()));
    rlpOutput.writeLongScalar(transaction.getGasLimit());
    rlpOutput.writeBytes(getTo(transaction));
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getValue()));
    rlpOutput.writeBytes(transaction.getPayload());
    writeAccessList(rlpOutput, transaction);
    rlpOutput.endList();
  }

  private static void encodeDelegateCodeForSigning(
      final BytesValueRLPOutput rlpOutput, final Transaction transaction) {
    rlpOutput.writeRaw(Bytes.of(transaction.getType().getSerializedType()));
    rlpOutput.startList();
    rlpOutput.writeBigIntegerScalar(transaction.getChainId().orElseThrow());
    rlpOutput.writeLongScalar(transaction.getNonce());
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getMaxPriorityFeePerGas().orElseThrow()));
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getMaxFeePerGas().orElseThrow()));
    rlpOutput.writeLongScalar(transaction.getGasLimit());
    rlpOutput.writeBytes(getTo(transaction));
    rlpOutput.writeUInt256Scalar(toUInt256(transaction.getValue()));
    rlpOutput.writeBytes(transaction.getPayload());
    writeAccessList(rlpOutput, transaction);
    writeCodeDelegationList(rlpOutput, transaction);
    rlpOutput.endList();
  }

  private static UInt256 toUInt256(final Quantity quantity) {
    return UInt256.valueOf(quantity.getAsBigInteger());
  }

  private static Address getTo(final Transaction transaction) {
    return transaction
        .getTo()
        .map(addr -> Address.wrap(Bytes.wrap(addr.toArrayUnsafe())))
        .orElse(Address.ZERO);
  }

  private static void writeAccessList(final RLPOutput rlpOutput, final Transaction transaction) {
    rlpOutput.startList();
    if (transaction instanceof org.hyperledger.besu.ethereum.core.Transaction coreTx) {
      coreTx
          .getAccessList()
          .ifPresent(
              accessList ->
                  accessList.forEach(
                      entry -> {
                        rlpOutput.startList();
                        rlpOutput.writeBytes(entry.address());
                        rlpOutput.startList();
                        entry.storageKeys().forEach(rlpOutput::writeBytes);
                        rlpOutput.endList();
                        rlpOutput.endList();
                      }));
    }
    rlpOutput.endList();
  }

  private static void writeCodeDelegationList(
      final RLPOutput rlpOutput, final Transaction transaction) {
    rlpOutput.startList();
    if (transaction instanceof org.hyperledger.besu.ethereum.core.Transaction coreTx) {
      coreTx
          .getCodeDelegationList()
          .ifPresent(
              delegationList ->
                  delegationList.forEach(
                      delegation -> {
                        rlpOutput.startList();
                        rlpOutput.writeBigIntegerScalar(delegation.chainId());
                        rlpOutput.writeBytes(delegation.address());
                        rlpOutput.writeLongScalar(delegation.nonce());
                        rlpOutput.writeIntScalar(delegation.v());
                        rlpOutput.writeBigIntegerScalar(delegation.r());
                        rlpOutput.writeBigIntegerScalar(delegation.s());
                        rlpOutput.endList();
                      }));
    }
    rlpOutput.endList();
  }
}
