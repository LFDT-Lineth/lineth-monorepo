/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.tx.gas.DefaultGasProvider;
import org.web3j.utils.Numeric;

public class TransactionTraceLimitOverflowLimitlessTest extends LineaPluginTestBase {

  private static final BigInteger GAS_LIMIT = DefaultGasProvider.GAS_LIMIT;
  private static final BigInteger VALUE = BigInteger.ZERO;
  private static final BigInteger GAS_PRICE = BigInteger.TEN.pow(11);

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        // set the module limits file
        .set(
            "--plugin-linea-module-limit-file-path=",
            getResourcePath("/strictModuleLimitsLimitless.toml"))
        // enabled the ZkCounter
        .set("--plugin-linea-limitless-enabled=", "true")
        .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "false")
        .build();
  }

  @Test
  public void transactionOverModuleLineCountRemoved() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final String txData = Bytes.repeat((byte) 3, 2000).toUnprefixedHexString();

    // this tx will not be selected since it goes above the max block size,
    // but selection should go on and select the next one
    final RawTransaction txModuleLineCountTooBig =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.ZERO,
            GAS_LIMIT.divide(BigInteger.TEN),
            accounts.getPrimaryBenefactor().getAddress(),
            VALUE,
            txData,
            GAS_PRICE,
            GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));
    final byte[] signedTxContractInteraction =
        TransactionEncoder.signMessage(
            txModuleLineCountTooBig, Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY));
    final EthSendTransaction signedTxContractInteractionResp =
        web3j.ethSendRawTransaction(Numeric.toHexString(signedTxContractInteraction)).send();

    // these are under the block size limit and should be selected
    final Account fewLinesSender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.createAccount("recipient");
    final List<Hash> expectedConfirmedTxs = new ArrayList<>(4);

    expectedConfirmedTxs.addAll(
        minerNode.execute(
            accountTransactions.createIncrementalTransfers(fewLinesSender, recipient, 4)));

    expectedConfirmedTxs.stream()
        .map(Hash::toHexString)
        .forEach(hash -> minerNode.verify(eth.expectSuccessfulTransactionReceipt(hash)));

    // assert that tx over line count limit is not confirmed and is removed from the pool
    minerNode.verify(
        eth.expectNoTransactionReceipt(signedTxContractInteractionResp.getTransactionHash()));
    assertTransactionNotInThePool(signedTxContractInteractionResp.getTransactionHash());
  }
}
