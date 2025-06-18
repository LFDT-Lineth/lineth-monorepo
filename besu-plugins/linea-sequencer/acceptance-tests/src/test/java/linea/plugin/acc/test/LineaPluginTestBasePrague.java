/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package linea.plugin.acc.test;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.google.common.io.Resources;
import java.io.File;
import java.io.IOException;
import java.math.BigInteger;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.Collection;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Response;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.consensus.clique.CliqueExtraData;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.ethereum.eth.transactions.ImmutableTransactionPoolConfiguration;
import org.hyperledger.besu.ethereum.eth.transactions.TransactionPoolConfiguration;
import org.hyperledger.besu.tests.acceptance.dsl.EngineAPIService;
import org.hyperledger.besu.tests.acceptance.dsl.node.RunnableNode;
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory;
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory.CliqueOptions;
import org.junit.jupiter.api.BeforeEach;
import org.web3j.crypto.Blob;
import org.web3j.crypto.BlobUtils;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.DefaultBlockParameterName;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.tx.gas.DefaultGasProvider;
import org.web3j.utils.Numeric;

// This file initializes a Besu node configured for the Prague fork and makes it available to
// acceptance tests.
@Slf4j
public abstract class LineaPluginTestBasePrague extends LineaPluginTestBase {
  private EngineAPIService engineApiService;
  protected ObjectMapper mapper;

  private static final BigInteger GAS_PRICE = DefaultGasProvider.GAS_PRICE;
  private static final BigInteger GAS_LIMIT = DefaultGasProvider.GAS_LIMIT;
  private static final BigInteger VALUE = BigInteger.ZERO;
  private static final String DATA = "0x";

  // Override this in subclasses to use a different genesis file template
  protected String getGenesisFileTemplatePath() {
    return "/clique/clique-prague-no-blobs.json.tpl";
  }

  @BeforeEach
  @Override
  public void setup() throws Exception {
    minerNode =
        createCliqueNodeWithExtraCliOptionsAndRpcApis(
            "miner1",
            getCliqueOptions(),
            getTestCliOptions(),
            Set.of("LINEA", "MINER"),
            true,
            DEFAULT_REQUESTED_PLUGINS);
    minerNode.setTransactionPoolConfiguration(
        ImmutableTransactionPoolConfiguration.builder()
            .from(TransactionPoolConfiguration.DEFAULT)
            .noLocalPriority(true)
            .build());
    cluster.start(minerNode);
    mapper = new ObjectMapper();
    this.engineApiService = new EngineAPIService(minerNode, ethTransactions, mapper);
  }

  // Ideally GenesisConfigurationFactory.createCliqueGenesisConfig would support a custom genesis
  // file
  // path. We have resorted to inlining its logic here to allow a flexible genesis file path.
  @Override
  protected String provideGenesisConfig(
      final Collection<? extends RunnableNode> validators, final CliqueOptions cliqueOptions) {
    // Target state
    final String genesisTemplate =
        GenesisConfigurationFactory.readGenesisFile(getGenesisFileTemplatePath());
    final String hydratedGenesisTemplate =
        genesisTemplate
            .replace("%blockperiodseconds%", String.valueOf(cliqueOptions.blockPeriodSeconds()))
            .replace("%epochlength%", String.valueOf(cliqueOptions.epochLength()))
            .replace("%createemptyblocks%", String.valueOf(cliqueOptions.createEmptyBlocks()));

    final List<Address> addresses =
        validators.stream().map(RunnableNode::getAddress).collect(Collectors.toList());
    final String extraDataString = CliqueExtraData.createGenesisExtraDataString(addresses);
    final String genesis = hydratedGenesisTemplate.replaceAll("%extraData%", extraDataString);

    return maybeCustomGenesisExtraData()
        .map(ed -> setGenesisCustomExtraData(genesis, ed))
        .orElse(genesis);
  }

  // No-arg override for simple test cases, we take sensible defaults from the genesis config
  protected void buildNewBlock() throws IOException, InterruptedException {
    var latestTimestamp = this.minerNode.execute(ethTransactions.block()).getTimestamp();
    var genesisConfigSerialized = this.minerNode.getGenesisConfig().get();
    JsonNode genesisConfig = mapper.readTree(genesisConfigSerialized);
    long defaultSlotTimeSeconds =
        genesisConfig.path("config").path("clique").path("blockperiodseconds").asLong();
    this.engineApiService.buildNewBlock(
        latestTimestamp.longValue() + defaultSlotTimeSeconds, defaultSlotTimeSeconds * 1000);
  }

  // @param blockTimestampSeconds    The Unix timestamp (in seconds) to assign to the new block.
  // @param blockBuildingTimeMs      The duration (in milliseconds) allocated for the Besu node to
  // build the block.
  protected void buildNewBlock(long blockTimestampSeconds, long blockBuildingTimeMs)
      throws IOException, InterruptedException {
    this.engineApiService.buildNewBlock(blockTimestampSeconds, blockBuildingTimeMs);
  }

  /**
   * Creates and sends a blob transaction. This method is designed to be stateless and should not
   * rely on any class properties or instance methods. All required data should be passed as
   * parameters. This makes it easier to test and reuse in different contexts.
   */
  protected EthSendTransaction sendRawBlobTransaction(
      Web3j web3j, Credentials credentials, String recipient) throws IOException {
    BigInteger nonce =
        web3j
            .ethGetTransactionCount(credentials.getAddress(), DefaultBlockParameterName.PENDING)
            .send()
            .getTransactionCount();

    // Take blob file from public reference so we can sanity check values -
    // https://github.com/LFDT-web3j/web3j/blob/9dbd2f90468538408eeb9a1e87e8e73a9f3dda3b/crypto/src/test/java/org/web3j/crypto/BlobUtilsTest.java#L63-L83
    URL blobUrl = new File(getResourcePath("/blob.txt")).toURI().toURL();
    final var blobHexString = Resources.toString(blobUrl, StandardCharsets.UTF_8);
    final Blob blob = new Blob(Numeric.hexStringToByteArray(blobHexString));
    final Bytes kzgCommitment = BlobUtils.getCommitment(blob);
    final Bytes kzgProof = BlobUtils.getProof(blob, kzgCommitment);
    final Bytes versionedHash = BlobUtils.kzgToVersionedHash(kzgCommitment);
    final RawTransaction rawTransaction =
        RawTransaction.createTransaction(
            List.of(blob),
            List.of(kzgCommitment),
            List.of(kzgProof),
            CHAIN_ID,
            nonce,
            GAS_PRICE,
            GAS_PRICE,
            GAS_LIMIT,
            recipient,
            VALUE,
            DATA,
            BigInteger.ONE,
            List.of(versionedHash));
    byte[] signedMessage = TransactionEncoder.signMessage(rawTransaction, credentials);
    String hexValue = Numeric.toHexString(signedMessage);
    return web3j.ethSendRawTransaction(hexValue).send();
  }

  protected Response importPremadeBlock(
      final ObjectNode executionPayload,
      final ArrayNode expectedBlobVersionedHashes,
      final String parentBeaconBlockRoot,
      final ArrayNode executionRequests)
      throws IOException, InterruptedException {
    return this.engineApiService.importPremadeBlock(
        executionPayload, expectedBlobVersionedHashes, parentBeaconBlockRoot, executionRequests);
  }
}
