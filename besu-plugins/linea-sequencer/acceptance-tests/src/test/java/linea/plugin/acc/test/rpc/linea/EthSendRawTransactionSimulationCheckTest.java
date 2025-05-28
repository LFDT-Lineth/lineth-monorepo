/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea;

import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.stream.IntStream;

import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import linea.plugin.acc.test.tests.web3j.generated.ExcludedPrecompiles;
import linea.plugin.acc.test.tests.web3j.generated.MulmodExecutor;
import linea.plugin.acc.test.tests.web3j.generated.RevertExample;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransactionSet;
import org.junit.jupiter.api.Test;
import org.web3j.abi.datatypes.generated.Bytes8;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.tx.gas.DefaultGasProvider;
import org.web3j.utils.Numeric;

public class EthSendRawTransactionSimulationCheckTest extends LineaPluginTestBase {
  private static final BigInteger GAS_LIMIT = DefaultGasProvider.GAS_LIMIT;
  private static final BigInteger VALUE = BigInteger.ZERO;
  private static final BigInteger GAS_PRICE = BigInteger.TEN.pow(9);

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        .set(
            "--plugin-linea-module-limit-file-path=",
            getResourcePath("/moduleLimits_sendRawTx.toml"))
        .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "true")
        .build();
  }

  @Test
  public void transactionOverModuleLineCountNotAccepted() throws Exception {
    final var mulmodExecutor = deployMulmodExecutor();

    final var mulmodOverflow =
        encodedCallMulmodOperation(mulmodExecutor, accounts.getPrimaryBenefactor(), 1, 5_000);

    final Web3j web3j = minerNode.nodeRequests().eth();
    final var resp = web3j.ethSendRawTransaction(Numeric.toHexString(mulmodOverflow)).send();
    assertThat(resp.hasError()).isTrue();
    assertThat(resp.getError().getMessage())
        .isEqualTo(
            "Transaction 0x6928439fd82ddf40709238e2df0f54ab2e51b252404fbf0efeebb515e6a405e0 line count for module EXT=33939 is above the limit 20");

    assertThat(getTxPoolContent()).isEmpty();
  }

  @Test
  public void validTransactionsAreAccepted() {
    // these are under the line count limit and should be accepted and selected
    final Account recipient = accounts.createAccount("recipient");
    final List<Hash> expectedConfirmedTxs = new ArrayList<>(4);

    final var transfers =
        IntStream.range(0, 4)
            .mapToObj(
                i ->
                    accountTransactions.createTransfer(
                        accounts.getSecondaryBenefactor(), recipient, i + 1, BigInteger.valueOf(i)))
            .toList()
            .reversed();
    // reversed, so we are sure no tx is selected before all are sent due to the nonce gap,
    // otherwise a block can be built with some txs before we can check the txpool content

    expectedConfirmedTxs.addAll(minerNode.execute(new TransferTransactionSet(transfers)));

    final var txPoolContentByHash = getTxPoolContent().stream().map(e -> e.get("hash")).toList();
    assertThat(txPoolContentByHash)
        .containsExactlyInAnyOrderElementsOf(
            expectedConfirmedTxs.stream().map(Hash::toHexString).toList());

    expectedConfirmedTxs.stream()
        .map(Hash::toHexString)
        .forEach(hash -> minerNode.verify(eth.expectSuccessfulTransactionReceipt(hash)));
  }

  @Test
  public void transactionsThatRevertAreAccepted() throws Exception {
    final RevertExample revertExample = deployRevertExample();
    final Web3j web3j = minerNode.nodeRequests().eth();
    final String contractAddress = revertExample.getContractAddress();
    final String txData = revertExample.setValue(BigInteger.ZERO).encodeFunctionCall();

    // this tx reverts but nevertheless it is accepted in the pool
    final RawTransaction txThatReverts =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.ZERO,
            GAS_LIMIT.divide(BigInteger.TEN),
            contractAddress,
            VALUE,
            txData,
            GAS_PRICE,
            GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));
    final byte[] signedTxContractInteraction =
        TransactionEncoder.signMessage(
            txThatReverts, Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY));

    final EthSendTransaction signedTxContractInteractionResp =
        web3j.ethSendRawTransaction(Numeric.toHexString(signedTxContractInteraction)).send();

    assertThat(signedTxContractInteractionResp.hasError()).isFalse();

    final var expectedConfirmedTxHash = signedTxContractInteractionResp.getTransactionHash();

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(expectedConfirmedTxHash));
  }

  @Test
  public void transactionsWithExcludedPrecompilesAreNotAccepted() throws Exception {
    final ExcludedPrecompiles excludedPrecompiles = deployExcludedPrecompiles();
    final Web3j web3j = minerNode.nodeRequests().eth();
    final String contractAddress = excludedPrecompiles.getContractAddress();

    record InvalidCall(String encodedContractCall, String expectedErrorMessage) {}

    final InvalidCall[] invalidCalls = {
      new InvalidCall(
          excludedPrecompiles
              .callRIPEMD160("I am not allowed here".getBytes(StandardCharsets.UTF_8))
              .encodeFunctionCall(),
          "Transaction 0x35451c83b480b45df19105a30f22704df8750b7e328e1ebc646e6442f2f426f9 line count for module PRECOMPILE_RIPEMD_BLOCKS=1 is above the limit 0"),
      new InvalidCall(
          encodedCallBlake2F(excludedPrecompiles),
          "Transaction 0xfd447b2b688f7448c875f68d9c85ffcb976e1cc722b70dae53e4f2e30d871be8 line count for module PRECOMPILE_BLAKE_ROUNDS=12 is above the limit 0")
    };

    Arrays.stream(invalidCalls)
        .forEach(
            invalidCall -> {
              // this tx must not be accepted
              final RawTransaction txInvalid =
                  RawTransaction.createTransaction(
                      CHAIN_ID,
                      BigInteger.ZERO,
                      GAS_LIMIT.divide(BigInteger.TEN),
                      contractAddress,
                      VALUE,
                      invalidCall.encodedContractCall,
                      GAS_PRICE,
                      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));

              final byte[] signedTxInvalid =
                  TransactionEncoder.signMessage(
                      txInvalid, Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY));

              final EthSendTransaction signedTxContractInteractionResp;
              try {
                signedTxContractInteractionResp =
                    web3j.ethSendRawTransaction(Numeric.toHexString(signedTxInvalid)).send();
              } catch (IOException e) {
                throw new RuntimeException(e);
              }

              assertThat(signedTxContractInteractionResp.hasError()).isTrue();
              assertThat(signedTxContractInteractionResp.getError().getMessage())
                  .isEqualTo(invalidCall.expectedErrorMessage);
            });
    assertThat(getTxPoolContent()).isEmpty();
  }

  protected byte[] encodedCallMulmodOperation(
      final MulmodExecutor executor, final Account sender, final int nonce, final int iterations) {
    final var operationCalldata =
        executor.executeMulmod(BigInteger.valueOf(iterations)).encodeFunctionCall();

    final var operationTx =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.valueOf(nonce),
            GAS_LIMIT,
            executor.getContractAddress(),
            BigInteger.ZERO,
            operationCalldata,
            GAS_PRICE,
            GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));

    return TransactionEncoder.signMessage(operationTx, sender.web3jCredentialsOrThrow());
  }

  private String encodedCallBlake2F(final ExcludedPrecompiles excludedPrecompiles) {
    return excludedPrecompiles
        .callBlake2f(
            BigInteger.valueOf(12),
            List.of(
                Bytes32.fromHexString(
                        "0x48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5")
                    .toArrayUnsafe(),
                Bytes32.fromHexString(
                        "0xd182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b")
                    .toArrayUnsafe()),
            List.of(
                Bytes32.fromHexString(
                        "0x6162630000000000000000000000000000000000000000000000000000000000")
                    .toArrayUnsafe(),
                Bytes32.ZERO.toArrayUnsafe(),
                Bytes32.ZERO.toArrayUnsafe(),
                Bytes32.ZERO.toArrayUnsafe()),
            List.of(Bytes8.DEFAULT.getValue(), Bytes8.DEFAULT.getValue()),
            true)
        .encodeFunctionCall();
  }
}
