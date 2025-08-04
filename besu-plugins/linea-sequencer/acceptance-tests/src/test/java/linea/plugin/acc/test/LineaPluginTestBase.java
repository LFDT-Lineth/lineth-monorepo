/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package linea.plugin.acc.test;

import static net.consensys.linea.metrics.LineaMetricCategory.*;
import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.io.IOException;
import java.math.BigInteger;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.*;
import java.util.stream.Collectors;
import linea.plugin.acc.test.tests.web3j.generated.*;
import linea.plugin.acc.test.utils.MemoryAppender;
import lombok.extern.slf4j.Slf4j;
import org.apache.commons.lang3.RandomStringUtils;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.eth.transactions.ImmutableTransactionPoolConfiguration;
import org.hyperledger.besu.ethereum.eth.transactions.TransactionPoolConfiguration;
import org.hyperledger.besu.metrics.prometheus.MetricsConfiguration;
import org.hyperledger.besu.plugin.services.metrics.MetricCategory;
import org.hyperledger.besu.tests.acceptance.dsl.AcceptanceTestBase;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.hyperledger.besu.tests.acceptance.dsl.condition.txpool.TxPoolConditions;
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode;
import org.hyperledger.besu.tests.acceptance.dsl.node.RunnableNode;
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.BesuNodeConfigurationBuilder;
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.NodeConfigurationFactory;
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory;
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory.CliqueOptions;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.txpool.TxPoolTransactions;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.RemoteCall;
import org.web3j.protocol.core.methods.response.TransactionReceipt;
import org.web3j.protocol.exceptions.TransactionException;
import org.web3j.tx.RawTransactionManager;
import org.web3j.tx.TransactionManager;
import org.web3j.tx.gas.DefaultGasProvider;
import org.web3j.tx.response.PollingTransactionReceiptProcessor;
import org.web3j.tx.response.TransactionReceiptProcessor;

/** Base class for plugin tests. */
@Slf4j
public abstract class LineaPluginTestBase extends AcceptanceTestBase {
  public static final int MAX_CALLDATA_SIZE = 1188; // contract has a call data size of 1160
  public static final int MAX_TX_GAS_LIMIT = DefaultGasProvider.GAS_LIMIT.intValue();
  public static final long CHAIN_ID = 1337L;
  public static final int BLOCK_PERIOD_SECONDS = 5;
  public static final CliqueOptions DEFAULT_LINEA_CLIQUE_OPTIONS =
      new CliqueOptions(BLOCK_PERIOD_SECONDS, CliqueOptions.DEFAULT.epochLength(), false);
  protected static final List<String> DEFAULT_REQUESTED_PLUGINS =
      List.of(
          "LineaExtraDataPlugin",
          "LineaEstimateGasEndpointPlugin",
          "LineaSetExtraDataEndpointPlugin",
          "LineaTransactionPoolValidatorPlugin",
          "LineaTransactionSelectorPlugin",
          "LineaBundleEndpointsPlugin",
          "ForwardBundlesPlugin",
          "LineaTransactionValidatorPlugin");

  protected static final HttpClient HTTP_CLIENT = HttpClient.newHttpClient();
  protected BesuNode minerNode;

  @BeforeEach
  public void setup() throws Exception {
    minerNode =
        createCliqueNodeWithExtraCliOptionsAndRpcApis(
            "miner1",
            getCliqueOptions(),
            getTestCliOptions(),
            Set.of("LINEA", "MINER"),
            false,
            DEFAULT_REQUESTED_PLUGINS);
    minerNode.setTransactionPoolConfiguration(
        ImmutableTransactionPoolConfiguration.builder()
            .from(TransactionPoolConfiguration.DEFAULT)
            .noLocalPriority(true)
            .build());
    cluster.start(minerNode);
  }

  protected List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder().build();
  }

  protected CliqueOptions getCliqueOptions() {
    return DEFAULT_LINEA_CLIQUE_OPTIONS;
  }

  @AfterEach
  public void stop() {
    cluster.stop();
    cluster.close();
    MemoryAppender.reset();
  }

  protected Optional<Bytes32> maybeCustomGenesisExtraData() {
    return Optional.empty();
  }

  protected BesuNode createCliqueNodeWithExtraCliOptionsAndRpcApis(
      final String name,
      final CliqueOptions cliqueOptions,
      final List<String> extraCliOptions,
      final Set<String> extraRpcApis,
      final boolean isEngineRpcEnabled,
      final List<String> requestedPlugins)
      throws IOException {
    final NodeConfigurationFactory node = new NodeConfigurationFactory();

    final var nodeConfBuilder =
        new BesuNodeConfigurationBuilder()
            .name(name)
            .miningEnabled()
            .jsonRpcConfiguration(node.createJsonRpcWithCliqueEnabledConfig(extraRpcApis))
            .webSocketConfiguration(node.createWebSocketEnabledConfig())
            .inProcessRpcConfiguration(node.createInProcessRpcConfiguration(extraRpcApis))
            .devMode(false)
            .jsonRpcTxPool()
            .engineRpcEnabled(isEngineRpcEnabled)
            .genesisConfigProvider(
                validators -> Optional.of(provideGenesisConfig(validators, cliqueOptions)))
            .extraCLIOptions(extraCliOptions)
            .metricsConfiguration(
                MetricsConfiguration.builder()
                    .enabled(true)
                    .port(0)
                    .metricCategories(
                        Set.of(
                            PRICING_CONF,
                            SEQUENCER_PROFITABILITY,
                            TX_POOL_PROFITABILITY,
                            SEQUENCER_LIVENESS))
                    .build())
            .requestedPlugins(requestedPlugins);

    return besu.create(nodeConfBuilder.build());
  }

  protected String provideGenesisConfig(
      final Collection<? extends RunnableNode> validators, final CliqueOptions cliqueOptions) {
    final var genesis =
        GenesisConfigurationFactory.createCliqueGenesisConfig(validators, cliqueOptions).get();

    return maybeCustomGenesisExtraData()
        .map(ed -> setGenesisCustomExtraData(genesis, ed))
        .orElse(genesis);
  }

  protected String setGenesisCustomExtraData(final String genesis, final Bytes32 customExtraData) {
    final var om = new ObjectMapper();
    final ObjectNode root;
    try {
      root = (ObjectNode) om.readTree(genesis);
    } catch (JsonProcessingException e) {
      throw new RuntimeException(e);
    }
    final var existingExtraData = Bytes.fromHexString(root.get("extraData").asText());
    final var updatedExtraData = Bytes.concatenate(customExtraData, existingExtraData.slice(32));
    root.put("extraData", updatedExtraData.toHexString());
    return root.toPrettyString();
  }

  protected void sendTransactionsWithGivenLengthPayload(
      final SimpleStorage simpleStorage,
      final List<String> accounts,
      final Web3j web3j,
      final int num) {
    final String contractAddress = simpleStorage.getContractAddress();
    final String txData =
        simpleStorage.set(RandomStringUtils.secure().nextAlphabetic(num)).encodeFunctionCall();
    final List<String> hashes = new ArrayList<>();
    accounts.forEach(
        a -> {
          final Credentials credentials = Credentials.create(a);
          TransactionManager txManager = new RawTransactionManager(web3j, credentials, CHAIN_ID);
          for (int i = 0; i < 5; i++) {
            try {
              hashes.add(
                  txManager
                      .sendTransaction(
                          DefaultGasProvider.GAS_PRICE,
                          DefaultGasProvider.GAS_LIMIT,
                          contractAddress,
                          txData,
                          BigInteger.ZERO)
                      .getTransactionHash());
            } catch (IOException e) {
              throw new RuntimeException(e);
            }
          }
        });

    assertTransactionsInCorrectBlocks(web3j, hashes, num);
  }

  private void assertTransactionsInCorrectBlocks(Web3j web3j, List<String> hashes, int num) {
    final HashMap<Long, Integer> txMap = new HashMap<>();
    TransactionReceiptProcessor receiptProcessor = createReceiptProcessor(web3j);

    // CallData for the transaction for empty String is 68 and grows in steps of 32 with (String
    // size / 32)
    final int maxTxs = MAX_CALLDATA_SIZE / (68 + ((num + 31) / 32) * 32);

    // Wait for transaction to be mined and check that there are no more than maxTxs per block
    hashes.forEach(
        h -> {
          final TransactionReceipt transactionReceipt;
          try {
            transactionReceipt = receiptProcessor.waitForTransactionReceipt(h);
          } catch (IOException | TransactionException e) {
            throw new RuntimeException(e);
          }

          final long blockNumber = transactionReceipt.getBlockNumber().longValue();
          txMap.compute(blockNumber, (b, n) -> n == null ? 1 : n + 1);

          // make sure that no block contained more than maxTxs
          assertThat(txMap.get(blockNumber)).isLessThanOrEqualTo(maxTxs);
        });
    // make sure that at least one block has maxTxs
    assertThat(txMap).containsValue(maxTxs);
  }

  protected SimpleStorage deploySimpleStorage() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<SimpleStorage> deploy =
        SimpleStorage.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  protected DummyAdder deployDummyAdder() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<DummyAdder> deploy =
        DummyAdder.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  protected MulmodExecutor deployMulmodExecutor() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<MulmodExecutor> deploy =
        MulmodExecutor.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  protected ModExp deployModExp() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<ModExp> deploy = ModExp.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  protected EcPairing deployEcPairing() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<EcPairing> deploy =
        EcPairing.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  protected EcAdd deployEcAdd() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<EcAdd> deploy = EcAdd.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  protected EcMul deployEcMul() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<EcMul> deploy = EcMul.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  protected EcRecover deployEcRecover() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<EcRecover> deploy =
        EcRecover.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  protected RevertExample deployRevertExample() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<RevertExample> deploy =
        RevertExample.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  protected AcceptanceTestToken deployAcceptanceTestToken() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    // 1000 AT tokens will be assigned to this account on deploy
    final Credentials credentials = accounts.getPrimaryBenefactor().web3jCredentialsOrThrow();
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<AcceptanceTestToken> deploy =
        AcceptanceTestToken.deploy(web3j, txManager, new DefaultGasProvider());
    final var contract = deploy.send();
    final var balance = contract.balanceOf(accounts.getPrimaryBenefactor().getAddress()).send();
    assertThat(balance).isEqualTo(1000);
    return contract;
  }

  protected ExcludedPrecompiles deployExcludedPrecompiles() throws Exception {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager =
        new RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j));

    final RemoteCall<ExcludedPrecompiles> deploy =
        ExcludedPrecompiles.deploy(web3j, txManager, new DefaultGasProvider());
    return deploy.send();
  }

  public static String getResourcePath(String resource) {
    return Objects.requireNonNull(LineaPluginTestBase.class.getResource(resource)).getPath();
  }

  protected void assertTransactionsMinedInSeparateBlocks(Web3j web3j, List<String> hashes)
      throws Exception {
    TransactionReceiptProcessor receiptProcessor = createReceiptProcessor(web3j);

    final HashSet<Long> blockNumbers = new HashSet<>();
    for (String hash : hashes) {
      TransactionReceipt receipt = receiptProcessor.waitForTransactionReceipt(hash);
      assertThat(receipt).isNotNull();
      boolean isAdded = blockNumbers.add(receipt.getBlockNumber().longValue());
      assertThat(isAdded).isEqualTo(true);
    }
  }

  protected void assertTransactionsMinedInSameBlock(Web3j web3j, List<String> hashes) {
    TransactionReceiptProcessor receiptProcessor = createReceiptProcessor(web3j);
    Set<Long> blockNumbers =
        hashes.stream()
            .map(
                hash -> {
                  try {
                    TransactionReceipt receipt = receiptProcessor.waitForTransactionReceipt(hash);
                    assertThat(receipt).isNotNull();
                    return receipt.getBlockNumber().longValue();
                  } catch (IOException | TransactionException e) {
                    throw new RuntimeException(e);
                  }
                })
            .collect(Collectors.toSet());

    assertThat(blockNumbers.size()).isEqualTo(1);
  }

  protected void assertTransactionNotInThePool(String hash) {
    minerNode.verify(
        new TxPoolConditions(new TxPoolTransactions())
            .notInTransactionPool(Hash.fromHexString(hash)));
  }

  protected List<Map<String, String>> getTxPoolContent() {
    return minerNode.execute(new TxPoolTransactions().getTxPoolContents());
  }

  private TransactionReceiptProcessor createReceiptProcessor(Web3j web3j) {
    return new PollingTransactionReceiptProcessor(
        web3j,
        Math.max(1000, DEFAULT_LINEA_CLIQUE_OPTIONS.blockPeriodSeconds() * 1000 / 5),
        DEFAULT_LINEA_CLIQUE_OPTIONS.blockPeriodSeconds() * 3);
  }

  protected String sendTransactionWithGivenLengthPayload(
      final String account, final Web3j web3j, final int num) throws IOException {
    String to = Address.fromHexString("fe3b557e8fb62b89f4916b721be55ceb828dbd73").toString();
    TransactionManager txManager = new RawTransactionManager(web3j, Credentials.create(account));

    return txManager
        .sendTransaction(
            DefaultGasProvider.GAS_PRICE,
            BigInteger.valueOf(MAX_TX_GAS_LIMIT),
            to,
            RandomStringUtils.secure().nextAlphabetic(num),
            BigInteger.ZERO)
        .getTransactionHash();
  }

  protected Bytes32 createExtraDataPricingField(
      final long fixedCostKWei, final long variableCostKWei, final long minGasPriceKWei) {
    final UInt32 fixed = UInt32.valueOf(BigInteger.valueOf(fixedCostKWei));
    final UInt32 variable = UInt32.valueOf(BigInteger.valueOf(variableCostKWei));
    final UInt32 min = UInt32.valueOf(BigInteger.valueOf(minGasPriceKWei));

    return Bytes32.rightPad(
        Bytes.concatenate(Bytes.of((byte) 1), fixed.toBytes(), variable.toBytes(), min.toBytes()));
  }

  protected double getMetricValue(
      final MetricCategory category,
      final String metricName,
      final List<Map.Entry<String, String>> labelValues)
      throws IOException, InterruptedException {

    final var metricsReq =
        HttpRequest.newBuilder().GET().uri(URI.create(minerNode.metricsHttpUrl().get())).build();

    final var respLines = HTTP_CLIENT.send(metricsReq, HttpResponse.BodyHandlers.ofLines());

    final var searchString =
        category.getApplicationPrefix().orElse("")
            + category.getName()
            + "_"
            + metricName
            + labelValues.stream()
                .map(lv -> lv.getKey() + "=\"" + lv.getValue() + "\"")
                .collect(Collectors.joining(",", "{", "}"));

    final var foundMetric =
        respLines.body().filter(line -> line.startsWith(searchString)).findFirst();

    return foundMetric
        .map(line -> line.substring(searchString.length()).trim())
        .map(Double::valueOf)
        .orElse(Double.NaN);
  }

  protected String getLog() {
    return MemoryAppender.getLog();
  }

  protected String getAndResetLog() {
    final var log = MemoryAppender.getLog();
    MemoryAppender.reset();
    return log;
  }

  protected byte[] encodedCallModExp(
      final ModExp modExp, final Account sender, final int nonce, final Bytes input) {
    final var modExpCalldata = modExp.callModExp(input.toArrayUnsafe()).encodeFunctionCall();

    final var modExpCall =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.valueOf(nonce),
            DefaultGasProvider.GAS_LIMIT,
            modExp.getContractAddress(),
            BigInteger.ZERO,
            modExpCalldata,
            DefaultGasProvider.GAS_PRICE,
            DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));

    return TransactionEncoder.signMessage(modExpCall, sender.web3jCredentialsOrThrow());
  }

  protected byte[] encodedCallEcPairing(
      final EcPairing ecPairing, final Account sender, final int nonce, final Bytes input) {
    final var ecPairingCalldata =
        ecPairing.callEcPairing(input.toArrayUnsafe()).encodeFunctionCall();

    final var ecPairingCall =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.valueOf(nonce),
            DefaultGasProvider.GAS_LIMIT,
            ecPairing.getContractAddress(),
            BigInteger.ZERO,
            ecPairingCalldata,
            DefaultGasProvider.GAS_PRICE,
            DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));

    return TransactionEncoder.signMessage(ecPairingCall, sender.web3jCredentialsOrThrow());
  }

  protected byte[] encodedCallEcAdd(
      final EcAdd ecAdd, final Account sender, final int nonce, final Bytes input) {
    final var ecAddCalldata = ecAdd.callEcAdd(input.toArrayUnsafe()).encodeFunctionCall();

    final var ecAddCall =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.valueOf(nonce),
            DefaultGasProvider.GAS_LIMIT,
            ecAdd.getContractAddress(),
            BigInteger.ZERO,
            ecAddCalldata,
            DefaultGasProvider.GAS_PRICE,
            DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));

    return TransactionEncoder.signMessage(ecAddCall, sender.web3jCredentialsOrThrow());
  }

  protected byte[] encodedCallEcMul(
      final EcMul ecMul, final Account sender, final int nonce, final Bytes input) {
    final var ecMulCalldata = ecMul.callEcMul(input.toArrayUnsafe()).encodeFunctionCall();

    final var ecMulCall =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.valueOf(nonce),
            DefaultGasProvider.GAS_LIMIT,
            ecMul.getContractAddress(),
            BigInteger.ZERO,
            ecMulCalldata,
            DefaultGasProvider.GAS_PRICE,
            DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));

    return TransactionEncoder.signMessage(ecMulCall, sender.web3jCredentialsOrThrow());
  }

  protected byte[] encodedCallEcRecover(
      final EcRecover ecRecover, final Account sender, final int nonce, final Bytes input) {
    final var ecRecoverCalldata =
        ecRecover.callEcRecover(input.toArrayUnsafe()).encodeFunctionCall();

    final var ecRecoverCall =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.valueOf(nonce),
            DefaultGasProvider.GAS_LIMIT,
            ecRecover.getContractAddress(),
            BigInteger.ZERO,
            ecRecoverCalldata,
            DefaultGasProvider.GAS_PRICE,
            DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));

    return TransactionEncoder.signMessage(ecRecoverCall, sender.web3jCredentialsOrThrow());
  }
}
