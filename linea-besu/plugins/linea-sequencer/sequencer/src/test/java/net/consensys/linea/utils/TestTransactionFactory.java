/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.utils;

import java.math.BigInteger;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SignatureAlgorithm;
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.rlp.BytesValueRLPOutput;

/** Factory for creating test transactions with sensible defaults. */
public class TestTransactionFactory {

  private static final SignatureAlgorithm SIGNATURE_ALGORITHM =
      SignatureAlgorithmFactory.getInstance();
  private static final BigInteger DEFAULT_PRIVATE_KEY =
      new BigInteger("8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63", 16);
  // Use a non-precompile address (precompiles are 0x01-0x0a)
  private static final Address DEFAULT_RECIPIENT =
      Address.fromHexString("0x000000000000000000000000000000000000dead");
  private static final BigInteger DEFAULT_CHAIN_ID = BigInteger.valueOf(59144);
  private static final long DEFAULT_GAS_LIMIT = 21000L;
  private static final Wei DEFAULT_GAS_PRICE = Wei.of(1_000_000_000);

  private final KeyPair keyPair;
  private long nonce;

  /** Creates a new factory with the default private key. */
  public TestTransactionFactory() {
    this(DEFAULT_PRIVATE_KEY);
  }

  /**
   * Creates a new factory with a custom private key.
   *
   * @param privateKey the private key to use for signing transactions
   */
  public TestTransactionFactory(final BigInteger privateKey) {
    this.keyPair =
        SIGNATURE_ALGORITHM.createKeyPair(SIGNATURE_ALGORITHM.createPrivateKey(privateKey));
    this.nonce = 0;
  }

  /**
   * Builds a legacy (type 0) transaction with default values and auto-incrementing nonce.
   *
   * @return a signed transaction
   */
  public Transaction createTransaction() {
    return createTransaction(DEFAULT_RECIPIENT);
  }

  /**
   * Builds a legacy (type 0) transaction with a specific recipient.
   *
   * @param recipient the recipient address
   * @return a signed transaction
   */
  public Transaction createTransaction(final Address recipient) {
    return Transaction.builder()
        .type(TransactionType.FRONTIER)
        .chainId(DEFAULT_CHAIN_ID)
        .nonce(nonce++)
        .gasLimit(DEFAULT_GAS_LIMIT)
        .gasPrice(DEFAULT_GAS_PRICE)
        .to(recipient)
        .value(Wei.ZERO)
        .payload(Bytes.EMPTY)
        .signAndBuild(keyPair);
  }

  /**
   * Builds a legacy (type 0) transaction with a custom payload and the default recipient.
   *
   * @param payload the transaction payload (call data)
   * @return a signed transaction
   */
  public Transaction createTransactionWithPayload(final Bytes payload) {
    return Transaction.builder()
        .type(TransactionType.FRONTIER)
        .chainId(DEFAULT_CHAIN_ID)
        .nonce(nonce++)
        .gasLimit(DEFAULT_GAS_LIMIT)
        .gasPrice(DEFAULT_GAS_PRICE)
        .to(DEFAULT_RECIPIENT)
        .value(Wei.ZERO)
        .payload(payload)
        .signAndBuild(keyPair);
  }

  /**
   * Builds a legacy (type 0) transaction with custom parameters.
   *
   * @param recipient the recipient address
   * @param value the value to transfer
   * @param gasLimit the gas limit
   * @return a signed transaction
   */
  public Transaction createTransaction(
      final Address recipient, final Wei value, final long gasLimit) {
    return Transaction.builder()
        .type(TransactionType.FRONTIER)
        .chainId(DEFAULT_CHAIN_ID)
        .nonce(nonce++)
        .gasLimit(gasLimit)
        .gasPrice(DEFAULT_GAS_PRICE)
        .to(recipient)
        .value(value)
        .payload(Bytes.EMPTY)
        .signAndBuild(keyPair);
  }

  /**
   * RLP-encodes a transaction to a hex string.
   *
   * @param tx the transaction to encode
   * @return the hex-encoded transaction
   */
  public static String encodeTransaction(final Transaction tx) {
    final BytesValueRLPOutput out = new BytesValueRLPOutput();
    tx.writeTo(out);
    return out.encoded().toHexString();
  }

  /**
   * Returns the sender address derived from the key pair.
   *
   * @return the sender address
   */
  public Address getSenderAddress() {
    return Address.extract(keyPair.getPublicKey());
  }

  /**
   * Returns the current nonce value (next transaction will use this nonce).
   *
   * @return the current nonce
   */
  public long getCurrentNonce() {
    return nonce;
  }

  /** Resets the nonce counter to zero. */
  public void resetNonce() {
    this.nonce = 0;
  }

  /** Returns the default recipient address used by this factory. */
  public static Address getDefaultRecipient() {
    return DEFAULT_RECIPIENT;
  }
}
