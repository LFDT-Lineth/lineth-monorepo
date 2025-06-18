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
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import okhttp3.Response;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.BlobGas;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.RequestType;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.parameters.EnginePayloadParameter;
import org.hyperledger.besu.ethereum.core.BlockHeader;
import org.hyperledger.besu.ethereum.core.Difficulty;
import org.hyperledger.besu.ethereum.core.Request;
import org.hyperledger.besu.ethereum.mainnet.BodyValidation;
import org.hyperledger.besu.ethereum.mainnet.MainnetBlockHeaderFunctions;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.methods.response.EthSendTransaction;

/**
 * Tests that verify the LineaTransactionValidationPlugin correctly rejects BLOB transactions from
 * being executed
 */
public class BlobTransactionDenialTest extends LineaPluginTestBasePrague {
  private Web3j web3j;
  private Credentials credentials;
  private String recipient;

  @Override
  protected String getGenesisFileTemplatePath() {
    // We cannot use clique-prague-zero-blobs because `config.blobSchedule.prague.max = 0` will
    // block all blob txs
    return "/clique/clique-prague-one-blob.json.tpl";
  }

  @Override
  @BeforeEach
  public void setup() throws Exception {
    super.setup();
    web3j = minerNode.nodeRequests().eth();
    credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    recipient = accounts.getSecondaryBenefactor().getAddress();
  }

  @Test
  public void blobTransactionsIsRejectedFromTransactionPool() throws Exception {
    // Act - Send a blob transaction to transaction pool
    EthSendTransaction response = sendRawBlobTransaction(web3j, credentials, recipient);
    // No need to build new block.

    // Assert
    assertThat(response.hasError()).isTrue();
    assertThat(response.getError().getMessage())
        .contains("Plugin has marked the transaction as invalid");
  }

  // Ideally the block import test would be conducted with two nodes as follows:
  // 1. Start an additional minimal node with Prague config
  // 2. Ensure additional node is peered to minerNode
  // 3. Send blob tx to additional node
  // 4. Construct block on additional node
  // 5. Send 'debug_getBadBlocks' RPC request to minerNode, confirm that block is rejected from
  // import
  //
  // However we are currently unable to run more than one node per test, due to the CLI options
  // being
  // singleton options and this implemented in dependency repository - linea tracer.
  // Thus simulate the block import as below:
  // 1. Create a premade block containing a blob tx
  // 2. Import the premade block using 'engine_newPayloadV4' Engine API call

  @Test
  public void blobTransactionsIsRejectedFromNodeImport() throws Exception {
    // Arrange
    EngineNewPayloadRequest blockWithBlobTxRequest = getBlockWithBlobTxRequest(mapper);

    // Act
    Response response =
        this.importPremadeBlock(
            blockWithBlobTxRequest.executionPayload(),
            blockWithBlobTxRequest.expectedBlobVersionedHashes(),
            blockWithBlobTxRequest.parentBeaconBlockRoot(),
            blockWithBlobTxRequest.executionRequests());

    // Assert
    JsonNode result = mapper.readTree(response.body().string()).get("result");
    String status = result.get("status").asText();
    String validationError = result.get("validationError").asText();
    assertThat(status).isEqualTo("INVALID");
    assertThat(validationError).contains("LineaTransactionValidatorPlugin - BLOB_TX_NOT_ALLOWED");
  }

  private record EngineNewPayloadRequest(
      ObjectNode executionPayload,
      ArrayNode expectedBlobVersionedHashes,
      String parentBeaconBlockRoot,
      ArrayNode executionRequests) {}

  private EngineNewPayloadRequest getBlockWithBlobTxRequest(ObjectMapper mapper) throws Exception {
    // Obtained following values by running `blobTransactionsIsRejectedFromTransactionPool` test
    // without the LineaTransactionSelectorPlugin and LineaTransactionValidatorPlugin plugins.
    Map<String, String> blockWithBlockTxParams = new HashMap<>();
    blockWithBlockTxParams.put(
        "STATE_ROOT", "0x2c1457760c057cf42f2d509648d725ec1f557b9d8729a5361e517952f91d050e");
    blockWithBlockTxParams.put(
        "LOGS_BLOOM",
        "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000");
    blockWithBlockTxParams.put(
        "RECEIPTS_ROOT", "0xeaa8c40899a61ae59615cf9985f5e2194f8fd2b57d273be63bde6733e89b12ab");
    blockWithBlockTxParams.put("EXTRA_DATA", "0x626573752032352e362e302d6c696e656131");
    blockWithBlockTxParams.put(
        "BLOB_TX",
        "0x03f8908205398084f461090084f46109008389544094627306090abab3a6e1400e9345bc60c78a8bef578080c001e1a0018ef96865998238a5e1783b6cafbc1253235d636f15d318f1fb50ef6a5b8f6a80a0576a95756f32ab705a22b591ab464d5affc8c1c7fcd14d777bac24d83bc44821a01f93b26f4f9989c3fe764f4a58d264bcd71b9deab72d6852f5dcdf19d55494f1");
    blockWithBlockTxParams.put(
        "BLOB_VERSIONED_HASH",
        "0x018ef96865998238a5e1783b6cafbc1253235d636f15d318f1fb50ef6a5b8f6a");
    blockWithBlockTxParams.put(
        "EXECUTION_REQUEST",
        "0x01a4664c40aacebd82a2db79f0ea36c06bc6a19adbb10a4a15bf67b328c9b101d09e5c6ee6672978fdad9ef0d9e2ceffaee99223555d8601f0cb3bcc4ce1af9864779a416e0000000000000000");
    blockWithBlockTxParams.put(
        "TRANSACTIONS_ROOT", "0x7a430a1c9da1f6e25ff8e6e96217c359784f3438dc1d983b4695355d66437f8f");
    blockWithBlockTxParams.put(
        "WITHDRAWALS_ROOT", "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421");
    blockWithBlockTxParams.put("GAS_LIMIT", "0x1ca35ef");
    blockWithBlockTxParams.put("GAS_USED", "0x5208");
    blockWithBlockTxParams.put("TIMESTAMP", "0x5");
    blockWithBlockTxParams.put("BASE_FEE_PER_GAS", "0x7");
    blockWithBlockTxParams.put("EXCESS_BLOB_GAS", "0x0");
    blockWithBlockTxParams.put("BLOB_GAS_USED", "0x20000");
    blockWithBlockTxParams.put("BLOCK_NUMBER", "0x1");
    blockWithBlockTxParams.put("FEE_RECIPIENT", Address.ZERO.toHexString());
    blockWithBlockTxParams.put("PREV_RANDAO", Hash.ZERO.toHexString());
    blockWithBlockTxParams.put("PARENT_BEACON_BLOCK_ROOT", Hash.ZERO.toHexString());
    blockWithBlockTxParams = Collections.unmodifiableMap(blockWithBlockTxParams);
    // Seems that the genesis block hash change with each run, despite a constant genesis file
    String genesisBlockHash = getLatestBlockHash();

    ObjectNode executionPayload =
        createExecutionPayload(mapper, genesisBlockHash, blockWithBlockTxParams);
    ArrayNode expectedBlobVersionedHashes =
        createBlobVersionedHashes(mapper, blockWithBlockTxParams);
    ArrayNode executionRequests = createExecutionRequests(mapper, blockWithBlockTxParams);
    // Compute block hash and update payload
    BlockHeader blockHeader = computeBlockHeader(executionPayload, mapper, blockWithBlockTxParams);
    updateExecutionPayloadWithBlockHash(executionPayload, blockHeader);
    return new EngineNewPayloadRequest(
        executionPayload,
        expectedBlobVersionedHashes,
        blockWithBlockTxParams.get("PARENT_BEACON_BLOCK_ROOT"),
        executionRequests);
  }

  private String getLatestBlockHash() throws Exception {
    return web3j
        .ethGetBlockByNumber(org.web3j.protocol.core.DefaultBlockParameterName.LATEST, false)
        .send()
        .getBlock()
        .getHash();
  }

  private ObjectNode createExecutionPayload(
      ObjectMapper mapper, String genesisBlockHash, Map<String, String> blockParams) {
    ObjectNode payload =
        mapper
            .createObjectNode()
            .put("parentHash", genesisBlockHash)
            .put("feeRecipient", blockParams.get("FEE_RECIPIENT"))
            .put("stateRoot", blockParams.get("STATE_ROOT"))
            .put("logsBloom", blockParams.get("LOGS_BLOOM"))
            .put("prevRandao", blockParams.get("PREV_RANDAO"))
            .put("gasLimit", blockParams.get("GAS_LIMIT"))
            .put("gasUsed", blockParams.get("GAS_USED"))
            .put("timestamp", blockParams.get("TIMESTAMP"))
            .put("extraData", blockParams.get("EXTRA_DATA"))
            .put("baseFeePerGas", blockParams.get("BASE_FEE_PER_GAS"))
            .put("excessBlobGas", blockParams.get("EXCESS_BLOB_GAS"))
            .put("blobGasUsed", blockParams.get("BLOB_GAS_USED"))
            .put("receiptsRoot", blockParams.get("RECEIPTS_ROOT"))
            .put("blockNumber", blockParams.get("BLOCK_NUMBER"));

    // Add transactions (blob tx)
    ArrayNode transactions = mapper.createArrayNode();
    transactions.add(blockParams.get("BLOB_TX"));
    payload.set("transactions", transactions);
    // Add withdrawals (empty list)
    ArrayNode withdrawals = mapper.createArrayNode();
    payload.set("withdrawals", withdrawals);

    return payload;
  }

  private ArrayNode createBlobVersionedHashes(
      ObjectMapper mapper, Map<String, String> blockParams) {
    ArrayNode hashes = mapper.createArrayNode();
    hashes.add(blockParams.get("BLOB_VERSIONED_HASH"));
    return hashes;
  }

  private ArrayNode createExecutionRequests(ObjectMapper mapper, Map<String, String> blockParams) {
    ArrayNode requests = mapper.createArrayNode();
    requests.add(blockParams.get("EXECUTION_REQUEST"));
    return requests;
  }

  private BlockHeader computeBlockHeader(
      ObjectNode executionPayload, ObjectMapper mapper, Map<String, String> blockParams)
      throws Exception {
    EnginePayloadParameter blockParam =
        mapper.readValue(executionPayload.toString(), EnginePayloadParameter.class);

    Hash transactionsRoot = Hash.fromHexString(blockParams.get("TRANSACTIONS_ROOT"));
    Hash withdrawalsRoot = Hash.fromHexString(blockParams.get("WITHDRAWALS_ROOT"));

    // Take code from AbstractEngineNewPayload in Besu codebase
    Bytes executionRequestBytes = Bytes.fromHexString(blockParams.get("EXECUTION_REQUEST"));
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
        Bytes32.fromHexString(blockParams.get("PARENT_BEACON_BLOCK_ROOT")),
        maybeRequests.map(BodyValidation::requestsHash).orElse(null),
        new MainnetBlockHeaderFunctions());
  }

  private void updateExecutionPayloadWithBlockHash(
      ObjectNode executionPayload, BlockHeader blockHeader) {
    executionPayload.put("blockHash", blockHeader.getBlockHash().toHexString());
  }
}
