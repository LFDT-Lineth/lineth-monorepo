/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea;

import static org.web3j.crypto.Hash.sha3;

import java.io.IOException;
import java.math.BigInteger;
import java.util.Arrays;
import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.tests.web3j.generated.AcceptanceTestToken;
import linea.plugin.acc.test.tests.web3j.generated.MulmodExecutor;
import lombok.RequiredArgsConstructor;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.protocol.core.Request;
import org.web3j.utils.Numeric;

public class AbstractSendBundleTest extends LineaPluginTestBase {
  protected static final BigInteger TRANSFER_GAS_LIMIT = BigInteger.valueOf(100_000L);
  protected static final BigInteger MULMOD_GAS_LIMIT = BigInteger.valueOf(9_000_000L);
  protected static final BigInteger GAS_PRICE = BigInteger.TEN.pow(9);

  protected TokenTransfer transferTokens(
      final AcceptanceTestToken token,
      final Account sender,
      final int nonce,
      final Account recipient,
      final int amount) {
    final var transferCalldata =
        token.transfer(recipient.getAddress(), BigInteger.valueOf(amount)).encodeFunctionCall();

    final var transferTx =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.valueOf(nonce),
            TRANSFER_GAS_LIMIT,
            token.getContractAddress(),
            BigInteger.ZERO,
            transferCalldata,
            GAS_PRICE,
            GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));

    final String signedTransferTx =
        Numeric.toHexString(
            TransactionEncoder.signMessage(transferTx, sender.web3jCredentialsOrThrow()));

    final String hashTx = sha3(signedTransferTx);

    return new TokenTransfer(recipient, hashTx, signedTransferTx);
  }

  protected MulmodCall mulmodOperation(
      final MulmodExecutor executor, final Account sender, final int nonce, final int iterations) {
    return mulmodOperation(executor, sender, nonce, iterations, MULMOD_GAS_LIMIT);
  }

  protected MulmodCall mulmodOperation(
      final MulmodExecutor executor,
      final Account sender,
      final int nonce,
      final int iterations,
      final BigInteger gasLimit) {
    final var operationCalldata =
        executor.executeMulmod(BigInteger.valueOf(iterations)).encodeFunctionCall();

    final var operationTx =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.valueOf(nonce),
            gasLimit,
            executor.getContractAddress(),
            BigInteger.ZERO,
            operationCalldata,
            GAS_PRICE,
            GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));

    final String signedTransferTx =
        Numeric.toHexString(
            TransactionEncoder.signMessage(operationTx, sender.web3jCredentialsOrThrow()));

    final String hashTx = sha3(signedTransferTx);

    return new MulmodCall(hashTx, signedTransferTx);
  }

  @RequiredArgsConstructor
  static class SendBundleRequest implements Transaction<SendBundleRequest.SendBundleResponse> {
    private final BundleParams bundleParams;

    @Override
    public SendBundleResponse execute(final NodeRequests nodeRequests) {
      try {
        return new Request<>(
                "linea_sendBundle",
                Arrays.asList(bundleParams),
                nodeRequests.getWeb3jService(),
                SendBundleResponse.class)
            .send();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }

    static class SendBundleResponse extends org.web3j.protocol.core.Response<Response> {}

    record Response(String bundleHash) {}
  }

  record BundleParams(String[] txs, String blockNumber) {}

  record TokenTransfer(Account recipient, String txHash, String rawTx) {}

  record MulmodCall(String txHash, String rawTx) {}
}
