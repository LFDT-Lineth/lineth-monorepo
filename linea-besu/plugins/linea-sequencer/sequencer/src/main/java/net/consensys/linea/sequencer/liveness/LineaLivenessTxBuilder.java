/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.FileInputStream;
import java.io.IOException;
import java.math.BigInteger;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Path;
import java.security.KeyStore;
import java.security.SignatureException;
import java.time.Duration;
import java.util.*;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
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
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.Sign;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.utils.Numeric;

@Slf4j
public class LineaLivenessTxBuilder implements LivenessTxBuilder {
  public static final BigInteger ZERO_TRANSACTION_VALUE = BigInteger.ZERO;
  private final HttpClient httpClient;
  private final ObjectMapper objectMapper;
  private final BlockchainService blockchainService;
  private final String signerKeyId;
  private final String signerUrl;
  private final String livenessContractAddress;
  private final long gasPrice;
  private final long gasLimit;
  private final BigInteger chainId;

  public LineaLivenessTxBuilder(
      final LineaLivenessServiceConfiguration lineaLivenessServiceConfiguration,
      final BlockchainService blockchainService,
      final BigInteger chainId) {
    this.chainId = chainId;
    this.signerKeyId = lineaLivenessServiceConfiguration.signerKeyId();
    this.signerUrl = lineaLivenessServiceConfiguration.signerUrl();
    this.livenessContractAddress = lineaLivenessServiceConfiguration.contractAddress();
    this.gasPrice = lineaLivenessServiceConfiguration.gasPrice();
    this.gasLimit = lineaLivenessServiceConfiguration.gasLimit();
    this.blockchainService = blockchainService;

    boolean tlsEnabled = lineaLivenessServiceConfiguration.tlsEnabled();
    Path tlsKeyStorePath = lineaLivenessServiceConfiguration.tlsKeyStorePath();
    String tlsKeyStorePassword = lineaLivenessServiceConfiguration.tlsKeyStorePassword();
    Path tlsTrustStorePath = lineaLivenessServiceConfiguration.tlsTrustStorePath();
    String tlsTrustStorePassword = lineaLivenessServiceConfiguration.tlsTrustStorePassword();

    // Build SSLContext instance
    Optional<SSLContext> sslContext =
        tlsEnabled
            ? buildSSLContext(
                tlsKeyStorePath, tlsKeyStorePassword, tlsTrustStorePath, tlsTrustStorePassword)
            : Optional.empty();

    // Initialize HTTP client and JSON mapper for Web3Signer API calls
    HttpClient.Builder httpBuilder = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(30));
    sslContext.ifPresent(httpBuilder::sslContext);

    httpClient = httpBuilder.build();
    objectMapper = new ObjectMapper();
  }

  private Optional<SSLContext> buildSSLContext(
      Path clientKeystorePath,
      String clientKeystorePassword,
      Path trustStorePath,
      String trustStorePassword) {
    try (FileInputStream keyStoreFis =
            new FileInputStream(clientKeystorePath.toAbsolutePath().toString());
        FileInputStream trustStoreFis =
            new FileInputStream(trustStorePath.toAbsolutePath().toString())) {
      // Load client keystore
      KeyStore keyStore = KeyStore.getInstance("PKCS12");
      keyStore.load(keyStoreFis, clientKeystorePassword.toCharArray());

      // Initialize KeyManagerFactory for client certificate
      KeyManagerFactory keyManagerFactory =
          KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
      keyManagerFactory.init(keyStore, clientKeystorePassword.toCharArray());

      // Load truststore
      KeyStore trustStore = KeyStore.getInstance("PKCS12");
      trustStore.load(trustStoreFis, trustStorePassword.toCharArray());

      // Initialize TrustManagerFactory for server certificate
      TrustManagerFactory trustManagerFactory =
          TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
      trustManagerFactory.init(trustStore);

      // Initialize SSLContext
      SSLContext sslContext = SSLContext.getInstance("TLS");
      sslContext.init(
          keyManagerFactory.getKeyManagers(), trustManagerFactory.getTrustManagers(), null);
      return Optional.of(sslContext);
    } catch (Exception ex) {
      throw new RuntimeException("Failed to initialize SSL context: " + ex.getMessage());
    }
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
    long adjustedGasPrice =
        Math.max(gasPrice, blockchainService.getNextBlockBaseFee().orElse(Wei.ONE).toLong());
    log.debug("Adjusted gas price: {} Wei (configured as {} Wei)", adjustedGasPrice, gasPrice);
    return Wei.of(adjustedGasPrice);
  }

  /**
   * Signs a raw transaction using Web3Signer.
   *
   * @param unsignedTransactionHex the hex of the encoded raw transaction to sign
   * @return the signed transaction
   */
  private String signTransactionWithWeb3Signer(String unsignedTransactionHex) throws IOException {
    try {
      // Prepare the request body for Web3Signer
      Map<String, String> requestBody = new HashMap<>();
      requestBody.put("data", unsignedTransactionHex);
      String jsonBody = objectMapper.writeValueAsString(requestBody);

      // Create HTTP request to Web3Signer
      String endpoint = signerUrl + "/api/v1/eth1/sign/" + signerKeyId;

      HttpRequest request =
          HttpRequest.newBuilder()
              .uri(URI.create(endpoint))
              .header("Content-Type", "application/json")
              .timeout(Duration.ofSeconds(30))
              .POST(HttpRequest.BodyPublishers.ofString(jsonBody))
              .build();

      // Send request and get response
      HttpResponse<String> response =
          httpClient.send(request, HttpResponse.BodyHandlers.ofString());

      if (response.statusCode() != 200) {
        String responseBody = response.body();
        String bodyDescription = responseBody != null ? responseBody : "<null>";
        throw new IOException(
            "Web3Signer API call failed with status: "
                + response.statusCode()
                + ", body: "
                + bodyDescription);
      }

      // The response should be the signed transaction hex string
      String responseBody = response.body();
      if (responseBody == null) {
        throw new IOException("Web3Signer API returned null response body");
      }

      String signedTransactionHex = responseBody.trim();

      if (signedTransactionHex.isEmpty()) {
        throw new IOException("Web3Signer API returned empty response body");
      }

      // Remove quotes if present (some APIs return quoted strings)
      if (signedTransactionHex.startsWith("\"") && signedTransactionHex.endsWith("\"")) {
        signedTransactionHex = signedTransactionHex.substring(1, signedTransactionHex.length() - 1);
      }

      log.debug("Successfully signed transaction with Web3Signer");
      return signedTransactionHex;

    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      throw new IOException("Web3Signer request was interrupted", e);
    }
  }

  /**
   * Signs a raw transaction using Web3Signer.
   *
   * @param rawTransaction the raw transaction to sign
   * @return the signed transaction
   * @throws IOException if signing fails, or the signed transaction is invalid
   */
  private Transaction signTransaction(RawTransaction rawTransaction) throws IOException {
    // Get the unsigned serialized transaction
    String unsignedTxEncodedHex = Numeric.toHexString(TransactionEncoder.encode(rawTransaction));

    String signedTxEncodedHash = signTransactionWithWeb3Signer(unsignedTxEncodedHex);

    // Additional validation layer (should not be needed due to signTransactionWithWeb3Signer
    // validation, but provides defense in depth)
    if (signedTxEncodedHash.trim().isEmpty()) {
      throw new IOException("Signed transaction hex is null or empty");
    }

    try {
      Sign.SignatureData signatureData = getSignatureData(signedTxEncodedHash);
      byte[] encodedSignedTxBytes = TransactionEncoder.encode(rawTransaction, signatureData);

      String encodedSignedTxHex = Numeric.toHexString(encodedSignedTxBytes);
      log.debug("encodedSignedTxHex: {}", encodedSignedTxHex);

      return DomainObjectDecodeUtils.decodeRawTransaction(encodedSignedTxHex);
    } catch (IllegalArgumentException e) {
      throw new IOException("Failed to parse signed transaction hex: " + e.getMessage(), e);
    } catch (Exception e) {
      throw new IOException("Unexpected error parsing signed transaction: " + e.getMessage(), e);
    }
  }

  private Sign.SignatureData getSignatureData(String signedTxEncodedHash)
      throws SignatureException {
    return Sign.signatureDataFromHex(signedTxEncodedHash);
  }
}
