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
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.protocol.Web3j;
import org.web3j.tx.RawTransactionManager;
import org.web3j.tx.TransactionManager;
import org.web3j.tx.gas.DefaultGasProvider;

/**
 * Example test using Besu node configured for Prague. Note that block building must be triggered
 * explicitly through `this.buildNewBlock()`
 */
public class ExamplePragueTest extends LineaPluginTestBasePrague {
  private static final BigInteger GAS_PRICE = DefaultGasProvider.GAS_PRICE;
  private static final BigInteger GAS_LIMIT = DefaultGasProvider.GAS_LIMIT;
  private static final BigInteger VALUE = BigInteger.ZERO;

  private Web3j web3j;
  private Credentials credentials;
  private TransactionManager txManager;
  private String recipient;

  @Override
  @BeforeEach
  public void setup() throws Exception {
    super.setup();
    web3j = minerNode.nodeRequests().eth();
    credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    txManager = new RawTransactionManager(web3j, credentials, CHAIN_ID);
    recipient = accounts.getSecondaryBenefactor().getAddress();
  }

  @Test
  public void legacyTransactionsAreAccepted() throws Exception {
    // Act - Send a legacy transaction
    String txHash =
        txManager
            .sendTransaction(GAS_PRICE, GAS_LIMIT, recipient, "0x", VALUE)
            .getTransactionHash();

    this.buildNewBlock();

    // Assert
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash));
  }

  @Test
  public void contractCreationTransactionsAreAccepted() throws Exception {
    // Act - Send a legacy transaction
    String txHash =
        txManager
            .sendTransaction(
                GAS_PRICE,
                GAS_LIMIT,
                null,
                "0x6080604052348015600e575f80fd5b50603e80601a5f395ff3fe60806040525f80fdfea2646970667358221220efe79e1e7d531be5f170d451c358bcde343b2b7a8bc35b84f0e8e0cbb00765a564736f6c634300081a0033",
                VALUE)
            .getTransactionHash();

    this.buildNewBlock();

    // Assert
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash));
  }
}
