/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.liveness;

import java.io.IOException;
import java.math.BigInteger;
import java.util.Arrays;
import java.util.Collections;
import java.util.Optional;
import java.util.function.Supplier;
import linea.crypto.Secp256k1Signature;
import linea.crypto.Signer;
import linea.web3j.ECKeypairSignerAdapter;
import lineth.config.LineaLivenessServiceConfiguration;
import lombok.extern.slf4j.Slf4j;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.api.util.DomainObjectDecodeUtils;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.web3j.abi.FunctionEncoder;
import org.web3j.abi.datatypes.Bool;
import org.web3j.abi.datatypes.Function;
import org.web3j.abi.datatypes.generated.Uint64;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.utils.Numeric;

@Slf4j
public class LineaLivenessTxBuilder implements LivenessTxBuilder {
  public static final BigInteger ZERO_TRANSACTION_VALUE = BigInteger.ZERO;
  private final Supplier<Optional<Wei>> nextBlockBaseFee;
  private final Signer<Secp256k1Signature> signer;
  private final String livenessContractAddress;
  private final long gasPrice;
  private final long gasLimit;
  private final BigInteger chainId;

  public LineaLivenessTxBuilder(
      final LineaLivenessServiceConfiguration lineaLivenessServiceConfiguration,
      final BlockchainService blockchainService,
      final BigInteger chainId) {
    this(
        lineaLivenessServiceConfiguration,
        blockchainService,
        chainId,
        new Web3SignerDigestSigner(lineaLivenessServiceConfiguration));
  }

  public LineaLivenessTxBuilder(
      final LineaLivenessServiceConfiguration lineaLivenessServiceConfiguration,
      final BlockchainService blockchainService,
      final BigInteger chainId,
      final Signer<Secp256k1Signature> signer) {
    this(
        lineaLivenessServiceConfiguration, blockchainService::getNextBlockBaseFee, chainId, signer);
  }

  LineaLivenessTxBuilder(
      final LineaLivenessServiceConfiguration lineaLivenessServiceConfiguration,
      final Supplier<Optional<Wei>> nextBlockBaseFee,
      final BigInteger chainId,
      final Signer<Secp256k1Signature> signer) {
    this.chainId = chainId;
    this.signer = signer;
    this.livenessContractAddress = lineaLivenessServiceConfiguration.contractAddress();
    this.gasPrice = lineaLivenessServiceConfiguration.gasPrice();
    this.gasLimit = lineaLivenessServiceConfiguration.gasLimit();
    this.nextBlockBaseFee = nextBlockBaseFee;
  }

  /**
   * Builds a transaction to update the LineaSequencerUptimeFeed contract.
   *
   * @param isUp true if the sequencer is up, false if it is down
   * @param timestamp the timestamp to report
   * @param nonce the nonce of the sender
   * @return Transaction
   * @throws IOException if there's an error creating, signing, or submitting the transaction after
   *     all retries
   */
  @Override
  public Transaction buildUptimeTransaction(boolean isUp, long timestamp, long nonce)
      throws IOException {
    Bytes callData = createFunctionCallData(isUp, timestamp);
    RawTransaction rawTransaction = createTransaction(callData, nonce);
    return signTransaction(rawTransaction);
  }

  /**
   * Creates the function call data for the LineaSequencerUptimeFeed contract.
   *
   * @param isUp true if the sequencer is up, false if it is down
   * @param timestamp the timestamp to report
   * @return the encoded function call data
   */
  private Bytes createFunctionCallData(boolean isUp, long timestamp) {
    Function function =
        new Function(
            "updateStatus",
            Arrays.asList(new Bool(!isUp), new Uint64(timestamp)),
            Collections.emptyList());

    String encodedFunction = FunctionEncoder.encode(function);
    byte[] callDataBytes = Numeric.hexStringToByteArray(encodedFunction);
    return Bytes.wrap(callDataBytes);
  }

  /**
   * Creates a raw transaction to call the LineaSequencerUptimeFeed contract.
   *
   * @param callData the encoded function call data
   * @param nonce the nonce of the signer
   * @return the raw transaction
   * @throws IOException if there's an error creating the transaction
   */
  private RawTransaction createTransaction(Bytes callData, long nonce) throws IOException {
    // Get gas price from configured value
    Wei gasPrice = getGasPrice();

    // Validate and get gas limit
    long gasLimit = this.gasLimit;

    // Create transaction
    return RawTransaction.createTransaction(
        chainId.longValue(),
        BigInteger.valueOf(nonce),
        BigInteger.valueOf(gasLimit),
        Address.fromHexString(livenessContractAddress).getBytes().toHexString(),
        ZERO_TRANSACTION_VALUE,
        callData.toHexString(),
        gasPrice.getAsBigInteger(),
        gasPrice.getAsBigInteger());
  }

  /**
   * Gets the gas price for transactions from the configured value.
   *
   * @return the gas price in Wei
   */
  private Wei getGasPrice() {
    // Use configured gas price
    long adjustedGasPrice = Math.max(gasPrice, nextBlockBaseFee.get().orElse(Wei.ONE).toLong());
    log.debug("Adjusted gas price: {} Wei (configured as {} Wei)", adjustedGasPrice, gasPrice);
    return Wei.of(adjustedGasPrice);
  }

  /**
   * Signs a raw transaction using the configured digest signer.
   *
   * @param rawTransaction the raw transaction to sign
   * @return the signed transaction
   * @throws IOException if signing fails, or the signed transaction is invalid
   */
  private Transaction signTransaction(RawTransaction rawTransaction) throws IOException {
    try {
      Credentials credentials = Credentials.create(new ECKeypairSignerAdapter(signer));
      byte[] encodedSignedTxBytes =
          TransactionEncoder.signMessage(rawTransaction, chainId.longValue(), credentials);
      String encodedSignedTxHex = Numeric.toHexString(encodedSignedTxBytes);
      return DomainObjectDecodeUtils.decodeRawTransaction(encodedSignedTxHex);
    } catch (Exception e) {
      if (e instanceof InterruptedException) {
        Thread.currentThread().interrupt();
      }
      throw new IOException("Failed to sign liveness transaction: " + e.getMessage(), e);
    }
  }
}
