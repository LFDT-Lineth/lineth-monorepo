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
import java.util.List;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SignatureAlgorithm;
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.CodeDelegation;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;

/** Utilities for creating EIP-7702 (delegate code) test fixtures. */
public final class EIP7702TestUtils {

  private static final SignatureAlgorithm SIGNATURE_ALGORITHM =
      SignatureAlgorithmFactory.getInstance();

  public static final BigInteger DEFAULT_CHAIN_ID = BigInteger.valueOf(59144);
  private static final long DEFAULT_GAS_LIMIT = 21000L;
  private static final Wei DEFAULT_MAX_FEE_PER_GAS = Wei.of(1_000_000_000);

  private EIP7702TestUtils() {}

  /**
   * Creates a KeyPair from a hex-encoded private key.
   *
   * @param privateKeyHex the private key as a hex string (without 0x prefix)
   * @return the generated KeyPair
   */
  public static KeyPair createKeyPair(final String privateKeyHex) {
    return SIGNATURE_ALGORITHM.createKeyPair(
        SIGNATURE_ALGORITHM.createPrivateKey(new BigInteger(privateKeyHex, 16)));
  }

  /**
   * Derives an address from a KeyPair.
   *
   * @param keyPair the KeyPair to derive the address from
   * @return the derived address
   */
  public static Address addressFromKeyPair(final KeyPair keyPair) {
    return Address.extract(keyPair.getPublicKey());
  }

  /**
   * Creates a signed CodeDelegation. The authorizer address is derived from the signing keypair.
   *
   * @param authorityKeyPair the keypair that signs the delegation (determines authorizer)
   * @param delegationTarget the address to delegate code execution to
   * @return a signed CodeDelegation
   */
  public static CodeDelegation createCodeDelegation(
      final KeyPair authorityKeyPair, final Address delegationTarget) {
    return createCodeDelegation(authorityKeyPair, delegationTarget, 0);
  }

  /**
   * Creates a signed CodeDelegation with a specific nonce.
   *
   * @param authorityKeyPair the keypair that signs the delegation (determines authorizer)
   * @param delegationTarget the address to delegate code execution to
   * @param nonce the nonce for the delegation
   * @return a signed CodeDelegation
   */
  private static CodeDelegation createCodeDelegation(
      final KeyPair authorityKeyPair, final Address delegationTarget, final long nonce) {
    // Fully-qualified: builder() lives on the core class, not the datatypes interface we import
    return org.hyperledger.besu.ethereum.core.CodeDelegation.builder()
        .chainId(DEFAULT_CHAIN_ID)
        .address(delegationTarget)
        .nonce(nonce)
        .signAndBuild(authorityKeyPair);
  }

  /**
   * Creates a signed EIP-7702 (delegate code) transaction.
   *
   * @param senderKeyPair the keypair that signs the transaction
   * @param recipient the recipient address
   * @param delegations the list of code delegations to include
   * @return a signed delegate code transaction
   */
  public static Transaction createDelegateCodeTransaction(
      final KeyPair senderKeyPair,
      final Address recipient,
      final List<CodeDelegation> delegations) {
    return createDelegateCodeTransaction(senderKeyPair, recipient, delegations, 0);
  }

  /**
   * Creates a signed EIP-7702 (delegate code) transaction with a specific nonce.
   *
   * @param senderKeyPair the keypair that signs the transaction
   * @param recipient the recipient address
   * @param delegations the list of code delegations to include
   * @param nonce the transaction nonce
   * @return a signed delegate code transaction
   */
  private static Transaction createDelegateCodeTransaction(
      final KeyPair senderKeyPair,
      final Address recipient,
      final List<CodeDelegation> delegations,
      final long nonce) {
    return Transaction.builder()
        .type(TransactionType.DELEGATE_CODE)
        .chainId(DEFAULT_CHAIN_ID)
        .nonce(nonce)
        .gasLimit(DEFAULT_GAS_LIMIT)
        .maxFeePerGas(DEFAULT_MAX_FEE_PER_GAS)
        .maxPriorityFeePerGas(DEFAULT_MAX_FEE_PER_GAS)
        .to(recipient)
        .value(Wei.ZERO)
        .payload(Bytes.EMPTY)
        .accessList(List.of())
        .codeDelegations(delegations)
        .signAndBuild(senderKeyPair);
  }
}
