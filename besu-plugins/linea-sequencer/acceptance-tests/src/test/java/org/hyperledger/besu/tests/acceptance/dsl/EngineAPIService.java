/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package org.hyperledger.besu.tests.acceptance.dsl;

import static org.assertj.core.api.Assertions.*;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.io.IOException;
import java.util.Optional;
import okhttp3.Call;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.eth.EthTransactions;
import org.web3j.crypto.BlobUtils;
import org.web3j.protocol.core.methods.response.EthBlock;

/*
 * Inspired by PragueAcceptanceTestHelper class in Besu codebase. We use this class to
 * emulate Engine API calls to the Besu Node, so that we can run tests for post-merge EVM forks.
 */
public class EngineAPIService {
  private final OkHttpClient httpClient;
  private final ObjectMapper mapper;
  private final BesuNode node;
  private final EthTransactions ethTransactions;

  private static final String JSONRPC_VERSION = "2.0";
  private static final long JSONRPC_REQUEST_ID = 67;

  public EngineAPIService(BesuNode node, EthTransactions ethTransactions, ObjectMapper mapper) {
    httpClient = new OkHttpClient();
    this.mapper = mapper;
    this.node = node;
    this.ethTransactions = ethTransactions;
  }

  /*
   * See https://hackmd.io/@danielrachi/engine_api
   *
   * The flow to build a block with the Engine API is as follows:
   * 1. Send engine_forkchoiceUpdated(EngineForkchoiceUpdatedParameter, EnginePayloadAttributesParameter) request to Besu node
   * 2. Besu node responds with payloadId
   * The Besu Node will start building a proposed block
   *
   * 3. Send engine_getPayload(payloadId) request to Besu node
   * 4. Besu node responds with executionPayload
   * Get the proposed block from the Besu node
   *
   * 5. Send engine_newPayload request to Besu node
   * Validate the proposed block. Then store the validated block for future reference.
   * Unsure why the proposed block is not stored in the previous steps where it was built.
   *
   * 6. Send engine_forkchoiceUpdated(EngineForkchoiceUpdatedParameter) request to Besu node
   * Add validated block to blockchain head.
   *
   * @param blockTimestampSeconds    The Unix timestamp (in seconds) to assign to the new block.
   * @param blockBuildingTimeMs      The duration (in milliseconds) allocated for the Besu node to build the block.
   */
  public void buildNewBlock(long blockTimestampSeconds, long blockBuildingTimeMs)
      throws IOException, InterruptedException {
    final EthBlock.Block latestBlock = node.execute(ethTransactions.block());

    final Call buildBlockRequest =
        createForkChoiceRequest(latestBlock.getHash(), blockTimestampSeconds);

    final String payloadId;
    try (final Response buildBlockResponse = buildBlockRequest.execute()) {
      // Ideally, we would deserialize directly into Besu native types such as
      // EngineUpdateForkchoiceResult and JsonRpcSuccessResponse. However, neither class
      // provides a default constructor or a constructor annotated with @JsonCreator.
      // As a result, deserializing them would require hefty boilerplate code (custom
      // deserializers and DTOs). To keep things simple and lightweight, we instead
      // parse the relevant fields manually from the expected JSON structure.
      payloadId =
          mapper
              .readTree(buildBlockResponse.body().string())
              .get("result")
              .get("payloadId")
              .asText();
      assertThat(payloadId).isNotEmpty();
    }

    // This is required to give the Besu node time to build the block. As per the Engine API spec,
    // engine_forkChoice will begin the payload build process and engine_getPayload may stop the
    // payload build process. Besu node behaviour is to stop the payload build process on
    // engine_getPayload. So unfortunately we lack a means to inspect a payload in-building without
    // interrupting it. Hence we must be conservative and wait for the 'SECONDS_PER_SLOT' time,
    // especially for slower machines running the tests.
    // See - https://github.com/ethereum/execution-apis/blob/main/src/engine/paris.md
    Thread.sleep(blockBuildingTimeMs);

    final Call getPayloadRequest = createGetPayloadRequest(payloadId);

    final ObjectNode executionPayload;
    final ObjectNode blobsBundle;
    final ArrayNode executionRequests;
    final String newBlockHash;
    final String parentBeaconBlockRoot;
    ArrayNode expectedBlobVersionedHashes = mapper.createArrayNode();
    try (final Response getPayloadResponse = getPayloadRequest.execute()) {
      assertThat(getPayloadResponse.code()).isEqualTo(200);
      JsonNode result = mapper.readTree(getPayloadResponse.body().string()).get("result");
      executionPayload = (ObjectNode) result.get("executionPayload");
      blobsBundle = (ObjectNode) result.get("blobsBundle");
      executionRequests = (ArrayNode) result.get("executionRequests");
      newBlockHash = executionPayload.get("blockHash").asText();
      parentBeaconBlockRoot = executionPayload.remove("parentBeaconBlockRoot").asText();
      // Transform KZG commitments to versioned hashes
      for (JsonNode kzgCommitment : blobsBundle.get("commitments")) {
        Bytes kzgBytes = Bytes.fromHexString(kzgCommitment.asText());
        expectedBlobVersionedHashes.add(BlobUtils.kzgToVersionedHash(kzgBytes).toString());
      }
      assertThat(newBlockHash).isNotEmpty();
    }

    final Call newPayloadRequest =
        createNewPayloadRequest(
            executionPayload,
            expectedBlobVersionedHashes,
            parentBeaconBlockRoot,
            executionRequests);

    try (final Response newPayloadResponse = newPayloadRequest.execute()) {
      assertThat(newPayloadResponse.code()).isEqualTo(200);
      final String responseStatus =
          mapper.readTree(newPayloadResponse.body().string()).get("result").get("status").asText();
      assertThat(responseStatus).isEqualTo("VALID");
    }

    final Call moveChainAheadRequest = createForkChoiceRequest(newBlockHash);

    try (final Response moveChainAheadResponse = moveChainAheadRequest.execute()) {
      assertThat(moveChainAheadResponse.code()).isEqualTo(200);
    }
  }

  public Response importPremadeBlock(
      final ObjectNode executionPayload,
      final ArrayNode expectedBlobVersionedHashes,
      final String parentBeaconBlockRoot,
      final ArrayNode executionRequests)
      throws IOException, InterruptedException {
    final Call newPayloadRequest =
        createNewPayloadRequest(
            executionPayload,
            expectedBlobVersionedHashes,
            parentBeaconBlockRoot,
            executionRequests);

    return newPayloadRequest.execute();
  }

  private Call createForkChoiceRequest(final String blockHash) {
    return createForkChoiceRequest(blockHash, null);
  }

  private Call createForkChoiceRequest(final String parentBlockHash, final Long blockTimestamp) {
    final Optional<Long> maybeTimeStamp = Optional.ofNullable(blockTimestamp);

    // Construct the first param - EngineForkchoiceUpdatedParameter
    ArrayNode params = mapper.createArrayNode();
    ObjectNode forkchoiceState = mapper.createObjectNode();
    forkchoiceState.put("headBlockHash", parentBlockHash);
    forkchoiceState.put("safeBlockHash", parentBlockHash);
    forkchoiceState.put("finalizedBlockHash", parentBlockHash);
    params.add(forkchoiceState);

    // Optionally construct the second param - EnginePayloadAttributesParameter
    if (maybeTimeStamp.isPresent()) {
      ObjectNode payloadAttributes = mapper.createObjectNode();
      payloadAttributes.put("timestamp", blockTimestamp);
      payloadAttributes.put("prevRandao", Hash.ZERO.toString());
      payloadAttributes.put("suggestedFeeRecipient", Address.ZERO.toString());
      payloadAttributes.set("withdrawals", mapper.createArrayNode());
      payloadAttributes.put("parentBeaconBlockRoot", Hash.ZERO.toString());
      params.add(payloadAttributes);
    }
    return createEngineCall("engine_forkchoiceUpdatedV3", params);
  }

  private Call createGetPayloadRequest(final String payloadId) {
    ArrayNode params = mapper.createArrayNode();
    params.add(payloadId);
    return createEngineCall("engine_getPayloadV4", params);
  }

  private Call createNewPayloadRequest(
      final ObjectNode executionPayload,
      final ArrayNode expectedBlobVersionedHashes,
      final String parentBeaconBlockRoot,
      final ArrayNode executionRequests) {
    ArrayNode params = mapper.createArrayNode();
    params.add(executionPayload);
    params.add(expectedBlobVersionedHashes);
    params.add(parentBeaconBlockRoot);
    params.add(executionRequests);

    return createEngineCall("engine_newPayloadV4", params);
  }

  private Call createEngineCall(final String rpcMethod, ArrayNode params) {
    ObjectNode request = mapper.createObjectNode();
    request.put("jsonrpc", JSONRPC_VERSION);
    request.put("method", rpcMethod);
    request.set("params", params);
    request.put("id", JSONRPC_REQUEST_ID);

    String requestString;
    try {
      requestString = mapper.writeValueAsString(request);
    } catch (Exception e) {
      throw new RuntimeException(
          "Failed to serialize JSON-RPC request for method " + rpcMethod + ":", e);
    }

    return httpClient.newCall(
        new Request.Builder()
            .url(node.engineRpcUrl().get())
            .post(
                RequestBody.create(
                    requestString, MediaType.parse("application/json; charset=utf-8")))
            .build());
  }
}
