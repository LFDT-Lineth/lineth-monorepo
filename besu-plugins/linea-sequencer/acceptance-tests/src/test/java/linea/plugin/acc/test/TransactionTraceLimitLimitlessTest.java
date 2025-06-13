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
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.tx.gas.DefaultGasProvider;
import org.web3j.utils.Numeric;

public class TransactionTraceLimitLimitlessTest extends LineaPluginTestBase {

  private static final BigInteger GAS_LIMIT = DefaultGasProvider.GAS_LIMIT;
  private static final BigInteger VALUE = BigInteger.ZERO;
  private static final BigInteger GAS_PRICE = BigInteger.TEN.pow(9);

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        // set the module limits file
        .set(
            "--plugin-linea-module-limit-file-path=",
            getResourcePath("/strictModuleLimitsLimitless.toml"))
        // enabled the ZkCounter
        .set("--plugin-linea-limitless-enabled=", "true")
        .build();
  }

  @Test
  public void transactionsMinedInSeparateBlocksTest() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);

    final String txData = Bytes.repeat((byte) 3, 1000).toUnprefixedHexString();

    // send txs that when encoded to RLP are bigger than 1000 byte, so only one should fit in a
    // block, since the
    // block size limit is 2000 byte
    final ArrayList<String> hashes = new ArrayList<>(5);
    for (int i = 0; i < 5; i++) {
      final RawTransaction transaction =
          RawTransaction.createTransaction(
              CHAIN_ID,
              BigInteger.valueOf(i),
              GAS_LIMIT,
              accounts.getSecondaryBenefactor().getAddress(),
              VALUE,
              txData,
              GAS_PRICE,
              GAS_PRICE.multiply(BigInteger.TEN));
      final byte[] signedTransaction = TransactionEncoder.signMessage(transaction, credentials);
      final EthSendTransaction response =
          web3j.ethSendRawTransaction(Numeric.toHexString(signedTransaction)).send();
      hashes.add(response.getTransactionHash());
    }

    // make sure that there are no more than one transaction per block, because the BLOCK_L1_SIZE
    // limit only allows for one of these transactions.
    assertTransactionsMinedInSeparateBlocks(web3j, hashes);
  }
}
