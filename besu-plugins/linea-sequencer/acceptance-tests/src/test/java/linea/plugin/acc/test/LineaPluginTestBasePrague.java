/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package linea.plugin.acc.test;

import static org.assertj.core.api.Assertions.assertThat;

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
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Response;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.consensus.clique.CliqueExtraData;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.BlobGas;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.RequestType;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.parameters.EnginePayloadParameter;
import org.hyperledger.besu.ethereum.core.BlockHeader;
import org.hyperledger.besu.ethereum.core.Difficulty;
import org.hyperledger.besu.ethereum.core.Request;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.eth.transactions.ImmutableTransactionPoolConfiguration;
import org.hyperledger.besu.ethereum.eth.transactions.TransactionPoolConfiguration;
import org.hyperledger.besu.ethereum.mainnet.BodyValidation;
import org.hyperledger.besu.ethereum.mainnet.MainnetBlockHeaderFunctions;
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
  private static final SECP256K1 secp256k1 = new SECP256K1();

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

  /**
   * Creates and sends EIP7702 delegate code transaction. This method is designed to be stateless
   * and should not rely on any class properties or instance methods. All required data should be
   * passed as parameters. This makes it easier to test and reuse in different contexts.
   */
  protected String sendRawEIP7702Transaction(Web3j web3j, Credentials credentials, String recipient)
      throws IOException {
    BigInteger nonce =
        web3j
            .ethGetTransactionCount(credentials.getAddress(), DefaultBlockParameterName.PENDING)
            .send()
            .getTransactionCount();

    // 7702 transaction
    final org.hyperledger.besu.datatypes.CodeDelegation codeDelegation =
        org.hyperledger.besu.ethereum.core.CodeDelegation.builder()
            .chainId(BigInteger.valueOf(CHAIN_ID))
            .address(Address.fromHexStringStrict(recipient))
            .nonce(1)
            .signAndBuild(
                secp256k1.createKeyPair(
                    secp256k1.createPrivateKey(credentials.getEcKeyPair().getPrivateKey())));

    final Transaction tx =
        Transaction.builder()
            .type(TransactionType.DELEGATE_CODE)
            .chainId(BigInteger.valueOf(CHAIN_ID))
            .nonce(nonce.longValue())
            .maxPriorityFeePerGas(Wei.of(GAS_PRICE))
            .maxFeePerGas(Wei.of(GAS_PRICE))
            .gasLimit(GAS_LIMIT.longValue())
            .to(Address.fromHexStringStrict(credentials.getAddress()))
            .value(Wei.ZERO)
            .payload(Bytes.EMPTY)
            .accessList(List.of())
            .codeDelegations(List.of(codeDelegation))
            .signAndBuild(
                secp256k1.createKeyPair(
                    secp256k1.createPrivateKey(credentials.getEcKeyPair().getPrivateKey())));

    String txHash =
        minerNode.execute(ethTransactions.sendRawTransaction(tx.encoded().toHexString()));
    return txHash;
  }

  /**
   * Imports a premade block using the Engine API to the test node.
   *
   * @param executionPayload Complete execution payload with block data
   * @param expectedBlobVersionedHashes Array of expected blob hashes
   * @param parentBeaconBlockRoot Root hash of the parent beacon block
   * @param executionRequests Array of execution layer requests
   * @return HTTP response from the Engine API call containing validation results
   * @throws IOException if the HTTP request fails
   * @throws InterruptedException if the request is interrupted
   */
  protected Response importPremadeBlock(
      final ObjectNode executionPayload,
      final ArrayNode expectedBlobVersionedHashes,
      final String parentBeaconBlockRoot,
      final ArrayNode executionRequests)
      throws IOException, InterruptedException {
    return this.engineApiService.importPremadeBlock(
        executionPayload, expectedBlobVersionedHashes, parentBeaconBlockRoot, executionRequests);
  }

  /**
   * Record containing all parameters required for engine_newPayloadV4 API calls.
   *
   * @param executionPayload ExecutionPayloadV3-compatible block data
   * @param expectedBlobVersionedHashes Array of 32-byte blob versioned hashes for validation
   * @param parentBeaconBlockRoot 32-byte root of the parent beacon block
   * @param executionRequests Array of execution layer triggered requests per EIP-7685
   */
  protected record EngineNewPayloadRequest(
      ObjectNode executionPayload,
      ArrayNode expectedBlobVersionedHashes,
      String parentBeaconBlockRoot,
      ArrayNode executionRequests) {}

  /**
   * Retrieves the hash of the latest block from the test node.
   *
   * @return The hexadecimal hash string of the latest block
   * @throws Exception if the RPC call to the node fails or returns invalid data
   */
  protected String getLatestBlockHash() throws Exception {
    return minerNode
        .nodeRequests()
        .eth()
        .ethGetBlockByNumber(org.web3j.protocol.core.DefaultBlockParameterName.LATEST, false)
        .send()
        .getBlock()
        .getHash();
  }

  /**
   * Creates an execution payload for block import testing.
   *
   * <p>Constructs an ExecutionPayloadV3-compatible JSON object that can be used with the
   * engine_newPayloadV4 method as defined in the Prague fork specification. The payload includes
   * all required block header fields and an optional transaction.
   *
   * @param mapper JSON object mapper for creating Jackson nodes
   * @param genesisBlockHash Hash of the genesis/parent block to reference
   * @param blockParams Map containing all block parameters using {@link BlockParams} constants as
   *     keys
   * @param transactionKey Key in blockParams map containing transaction data, or empty string for
   *     no transactions
   * @return ObjectNode representing the execution payload compatible with engine_newPayloadV4
   * @throws IllegalArgumentException if mapper, genesisBlockHash, blockParams, or transactionKey is
   *     null
   * @see <a href="https://github.com/ethereum/execution-apis/blob/main/src/engine/prague.md">Prague
   *     Engine API Specification</a>
   */
  protected ObjectNode createExecutionPayload(
      ObjectMapper mapper,
      String genesisBlockHash,
      Map<String, String> blockParams,
      String transactionKey) {
    ObjectNode payload =
        mapper
            .createObjectNode()
            .put("parentHash", genesisBlockHash)
            .put("feeRecipient", blockParams.get(BlockParams.FEE_RECIPIENT))
            .put("stateRoot", blockParams.get(BlockParams.STATE_ROOT))
            .put("logsBloom", blockParams.get(BlockParams.LOGS_BLOOM))
            .put("prevRandao", blockParams.get(BlockParams.PREV_RANDAO))
            .put("gasLimit", blockParams.get(BlockParams.GAS_LIMIT))
            .put("gasUsed", blockParams.get(BlockParams.GAS_USED))
            .put("timestamp", blockParams.get(BlockParams.TIMESTAMP))
            .put("extraData", blockParams.get(BlockParams.EXTRA_DATA))
            .put("baseFeePerGas", blockParams.get(BlockParams.BASE_FEE_PER_GAS))
            .put("excessBlobGas", blockParams.get(BlockParams.EXCESS_BLOB_GAS))
            .put("blobGasUsed", blockParams.get(BlockParams.BLOB_GAS_USED))
            .put("receiptsRoot", blockParams.get(BlockParams.RECEIPTS_ROOT))
            .put("blockNumber", blockParams.get(BlockParams.BLOCK_NUMBER));

    // Add transactions
    ArrayNode transactions = mapper.createArrayNode();
    if (blockParams.containsKey(transactionKey)) {
      transactions.add(blockParams.get(transactionKey));
    }
    payload.set("transactions", transactions);

    // Add withdrawals (empty list)
    ArrayNode withdrawals = mapper.createArrayNode();
    payload.set("withdrawals", withdrawals);

    return payload;
  }

  /**
   * Creates blob versioned hashes array from block parameters.
   *
   * <p>Extracts blob versioned hashes from block parameters for transactions that include blob
   * data. Each hash is 32 bytes and used to validate blob data integrity.
   *
   * @param mapper JSON object mapper for creating Jackson nodes
   * @param blockParams Map containing block parameters with blob hash data
   * @param versionedHashKey Key in blockParams for accessing the blob versioned hash
   * @return ArrayNode containing the blob versioned hash (32 bytes)
   * @throws IllegalArgumentException if mapper, blockParams, or versionedHashKey is null
   */
  protected ArrayNode createBlobVersionedHashes(
      ObjectMapper mapper, Map<String, String> blockParams, String versionedHashKey) {
    ArrayNode hashes = mapper.createArrayNode();
    if (blockParams.containsKey(versionedHashKey)) {
      hashes.add(blockParams.get(versionedHashKey));
    }
    return hashes;
  }

  /**
   * Creates an empty versioned hashes array for non-blob transactions.
   *
   * @param mapper JSON object mapper for creating Jackson nodes
   * @return Empty ArrayNode for blocks without blob transactions
   * @throws IllegalArgumentException if mapper is null
   */
  protected ArrayNode createEmptyVersionedHashes(ObjectMapper mapper) {
    return mapper.createArrayNode();
  }

  /**
   * Creates EIP-7685 execution requests array for Engine API block import.
   *
   * @param mapper JSON object mapper for creating Jackson nodes
   * @param blockParams Map containing block parameters with execution request data
   * @return ArrayNode containing execution requests as hex-encoded byte arrays
   */
  protected ArrayNode createExecutionRequests(
      ObjectMapper mapper, Map<String, String> blockParams) {
    ArrayNode requests = mapper.createArrayNode();
    requests.add(blockParams.get(BlockParams.EXECUTION_REQUEST));
    return requests;
  }

  /**
   * Computes a complete block header from execution payload and block parameters.
   *
   * <p>Creates a Besu BlockHeader instance that includes all required fields for Prague fork
   * including execution requests commitment. The computed header is used to generate the correct
   * blockHash for Engine API validation.
   *
   * @param executionPayload JSON execution payload created by {@link #createExecutionPayload}
   * @param mapper JSON object mapper for parsing the payload
   * @param blockParams Map containing all block parameters and roots
   * @return Complete BlockHeader instance with computed hash
   * @throws Exception if JSON parsing fails or block header construction fails
   */
  protected BlockHeader computeBlockHeader(
      ObjectNode executionPayload, ObjectMapper mapper, Map<String, String> blockParams)
      throws Exception {
    EnginePayloadParameter blockParam =
        mapper.readValue(executionPayload.toString(), EnginePayloadParameter.class);

    Hash transactionsRoot = Hash.fromHexString(blockParams.get(BlockParams.TRANSACTIONS_ROOT));
    Hash withdrawalsRoot = Hash.fromHexString(blockParams.get(BlockParams.WITHDRAWALS_ROOT));

    // Take code from AbstractEngineNewPayload in Besu codebase
    Bytes executionRequestBytes =
        Bytes.fromHexString(blockParams.get(BlockParams.EXECUTION_REQUEST));
    Bytes executionRequestBytesData = executionRequestBytes.slice(1);
    Request executionRequest =
        new Request(RequestType.of(executionRequestBytes.get(0)), executionRequestBytesData);
    Optional<List<Request>> maybeRequests = Optional.of(List.of(executionRequest));

    return new BlockHeader(
        blockParam.getParentHash(),
        Hash.EMPTY_LIST_HASH, // OMMERS_HASH_CONSTANT
        blockParam.getFeeRecipient(),
        blockParam.getStateRoot(),
        transactionsRoot,
        blockParam.getReceiptsRoot(),
        blockParam.getLogsBloom(),
        Difficulty.ZERO,
        blockParam.getBlockNumber(),
        blockParam.getGasLimit(),
        blockParam.getGasUsed(),
        blockParam.getTimestamp(),
        Bytes.fromHexString(blockParam.getExtraData()),
        blockParam.getBaseFeePerGas(),
        blockParam.getPrevRandao(),
        0, // Nonce
        withdrawalsRoot,
        blockParam.getBlobGasUsed(),
        BlobGas.fromHexString(blockParam.getExcessBlobGas()),
        Bytes32.fromHexString(blockParams.get(BlockParams.PARENT_BEACON_BLOCK_ROOT)),
        maybeRequests.map(BodyValidation::requestsHash).orElse(null),
        null, // BAL
        new MainnetBlockHeaderFunctions());
  }

  /**
   * Updates the execution payload with the computed block hash.
   *
   * @param executionPayload JSON execution payload to update
   * @param blockHeader Block header containing the computed hash
   */
  protected void updateExecutionPayloadWithBlockHash(
      ObjectNode executionPayload, BlockHeader blockHeader) {
    executionPayload.put("blockHash", blockHeader.getBlockHash().toHexString());
  }

  /**
   * Asserts that a block import was rejected with the expected validation error.
   *
   * @param response HTTP response from the Engine API call
   * @param expectedValidationError Expected validation error message to check for
   */
  protected void assertBlockImportRejected(Response response, String expectedValidationError)
      throws Exception {
    JsonNode result = mapper.readTree(response.body().string()).get("result");
    String status = result.get("status").asText();
    String validationError = result.get("validationError").asText();
    assertThat(status).isEqualTo("INVALID");
    assertThat(validationError).contains(expectedValidationError);
  }

  // Constants for transaction-related data keys
  public static final class TransactionDataKeys {
    public static final String BLOB_TX = "BLOB_TX";
    public static final String DELEGATE_CALL_TX = "DELEGATE_CALL_TX";
    public static final String BLOB_VERSIONED_HASH = "BLOB_VERSIONED_HASH";
  }

  // Constants for block parameter keys
  public static final class BlockParams {
    public static final String STATE_ROOT = "STATE_ROOT";
    public static final String LOGS_BLOOM = "LOGS_BLOOM";
    public static final String RECEIPTS_ROOT = "RECEIPTS_ROOT";
    public static final String EXTRA_DATA = "EXTRA_DATA";
    public static final String EXECUTION_REQUEST = "EXECUTION_REQUEST";
    public static final String TRANSACTIONS_ROOT = "TRANSACTIONS_ROOT";
    public static final String WITHDRAWALS_ROOT = "WITHDRAWALS_ROOT";
    public static final String GAS_LIMIT = "GAS_LIMIT";
    public static final String GAS_USED = "GAS_USED";
    public static final String TIMESTAMP = "TIMESTAMP";
    public static final String BASE_FEE_PER_GAS = "BASE_FEE_PER_GAS";
    public static final String EXCESS_BLOB_GAS = "EXCESS_BLOB_GAS";
    public static final String BLOB_GAS_USED = "BLOB_GAS_USED";
    public static final String BLOCK_NUMBER = "BLOCK_NUMBER";
    public static final String FEE_RECIPIENT = "FEE_RECIPIENT";
    public static final String PREV_RANDAO = "PREV_RANDAO";
    public static final String PARENT_BEACON_BLOCK_ROOT = "PARENT_BEACON_BLOCK_ROOT";
  }

  // Constants for validation error messages
  public static final class LineaTransactionValidatorPluginErrors {
    public static final String BLOB_TX_NOT_ALLOWED =
        "LineaTransactionValidatorPlugin - BLOB_TX_NOT_ALLOWED";
    public static final String DELEGATE_CODE_TX_NOT_ALLOWED =
        "LineaTransactionValidatorPlugin - DELEGATE_CODE_TX_NOT_ALLOWED";
  }
}
