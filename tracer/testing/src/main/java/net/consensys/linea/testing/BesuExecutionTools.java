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
package net.consensys.linea.testing;

import static com.google.common.base.Preconditions.checkArgument;
import static com.google.common.base.Preconditions.checkState;
import static java.lang.Long.parseLong;
import static net.consensys.linea.testing.ShomeiNode.MerkelProofResponse;
import static net.consensys.linea.zktracer.Fork.OSAKA;
import static org.assertj.core.api.Assertions.assertThat;
import static org.hyperledger.besu.tests.acceptance.dsl.WaitUtils.waitFor;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.databind.node.ArrayNode;
import io.netty.util.internal.ConcurrentSet;
import java.io.File;
import java.io.IOException;
import java.math.BigInteger;
import java.net.ServerSocket;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.*;
import java.util.concurrent.TimeUnit;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.corset.CorsetValidator;
import net.consensys.linea.plugins.rpc.tracegeneration.TraceFile;
import net.consensys.linea.plugins.rpc.tracegeneration.TraceRequestParams;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.json.JsonConverter;
import net.consensys.shomei.rpc.server.model.RollupGetZkEvmStateV0Parameter;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions;
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode;
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner;
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster;
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.eth.EthTransactions;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions;
import org.junit.jupiter.api.TestInfo;
import org.web3j.protocol.core.DefaultBlockParameter;
import org.web3j.protocol.core.methods.response.EthBlock.Block;

@Slf4j
public class BesuExecutionTools {

  private static final MediaType MEDIA_TYPE_JSON =
      MediaType.parse("application/json; charset=utf-8");
  private static final JsonConverter CONVERTER = JsonConverter.builder().build();
  private static final ObjectMapper MAPPER =
      CONVERTER.getObjectMapper().enable(SerializationFeature.INDENT_OUTPUT);

  private final OkHttpClient httpClient;
  private final BesuNode besuNode;
  private final ShomeiNode shomeiNode;
  private final Path testDataDir;
  private final Path shomeiDataPath;
  private final List<Transaction> transactions;
  private final String testName;
  private final GenesisConfigBuilder genesisConfigBuilder;
  private final Boolean oneTxPerBlock;

  public BesuExecutionTools(
      String testName,
      ChainConfig chainConfig,
      Address coinbase,
      List<ToyAccount> accounts,
      List<Transaction> transactions,
      Boolean oneTxPerBlock,
      String customGenesisFile) {
    String randomUUID = UUID.randomUUID().toString();
    String tmpTestName =
        Optional.ofNullable(testName)
            .map(name -> String.format("%s-%s", name, randomUUID))
            .orElse(randomUUID)
            .replaceAll("[,.:<>|*?\\r\\n\\[\\]() ]", "_");
    this.testName = tmpTestName.substring(0, Math.min(tmpTestName.length(), 200));
    int besuPort = findFreePort();
    int shomeiPort = findFreePort();
    this.httpClient = new OkHttpClient.Builder().readTimeout(30, TimeUnit.SECONDS).build();
    this.oneTxPerBlock = oneTxPerBlock;
    // Generate file per fork in testing/src/main/resources folder
    String genesisFileName =
        (customGenesisFile == null)
            ? "BesuExecutionToolsGenesis_" + chainConfig.fork.name() + ".json"
            : customGenesisFile;
    GenesisConfigBuilder genesisConfigBuilder = new GenesisConfigBuilder(genesisFileName);
    genesisConfigBuilder.setChainId(chainConfig.id);
    genesisConfigBuilder.setCoinbase(coinbase);
    genesisConfigBuilder.setGasLimit(chainConfig.gasLimitMaximum.longValue());
    accounts.forEach(genesisConfigBuilder::addAccount);
    this.genesisConfigBuilder = genesisConfigBuilder;
    try {
      this.testDataDir =
          Files.createDirectory(
              Path.of(System.getProperty("besu.traces.dir")).resolve(this.testName));
      this.shomeiDataPath = Files.createTempDirectory("shomei");
      this.besuNode =
          BesuNodeBuilder.create(
              testName,
              chainConfig.bridgeConfiguration,
              genesisConfigBuilder,
              besuPort,
              testDataDir,
              shomeiPort);

    } catch (IOException e) {
      throw new RuntimeException(e);
    }
    this.shomeiNode =
        new ShomeiNode.Builder()
            .setBesuRpcPort(besuPort)
            .setJsonRpcPort(shomeiPort)
            .setDataStoragePath(this.shomeiDataPath)
            .build();
    this.transactions = transactions;
  }

  private static String getTestName(Optional<TestInfo> testInfo) {
    return testInfo
        .map(
            info ->
                String.format(
                    "%s-%s-%s",
                    info.getTestClass().get().getSimpleName(),
                    info.getTestMethod().get().getName(),
                    info.getDisplayName()))
        .orElse(null);
  }

  public BesuExecutionTools(
      Optional<TestInfo> testInfo,
      ChainConfig chainConfig,
      Address coinbase,
      List<ToyAccount> accounts,
      List<Transaction> transactions,
      Boolean oneTxPerBlock,
      String customGenesisFile) {
    this(
        getTestName(testInfo),
        chainConfig,
        coinbase,
        accounts,
        transactions,
        oneTxPerBlock,
        customGenesisFile);
  }

  public void executeTest() {
    Thread shomeiThread = new Thread(shomeiNode);
    Cluster besuCluster =
        new Cluster(
            new ClusterConfigurationBuilder().build(),
            new NetConditions(new NetTransactions()),
            new ThreadBesuNodeRunner());
    try {

      checkArgument(
          !transactions.isEmpty(), "At least one transaction (including null) is required");

      shomeiThread.start();
      besuCluster.start(besuNode);

      EthTransactions ethTransactions = new EthTransactions();
      Map<String, Boolean> txReceiptProcessed = new HashMap<>();
      ConcurrentSet<Long> blockNumbers = new ConcurrentSet<>();
      List<String> txHashes = new ArrayList<>();
      Fork nextFork = null;
      Fork currentFork = nextFork;
      Iterator<Transaction> txs = transactions.iterator();
      Boolean txHasNext = txs.hasNext();
      long firstBlockNumber = 0;
      long finalBlockNumber = 0;

      while (txHasNext) {
        // Send transaction to the transaction pool with eth_sendRawTransaction
        // If oneTxPerBlock is true, we send one transaction per block
        if (oneTxPerBlock) {
          final Transaction tx = txs.next();
          if (tx != null) {
            String txHash =
                besuNode.execute(ethTransactions.sendRawTransaction(tx.encoded().toHexString()));
            txHashes.add(txHash);
          }
          txHasNext = txs.hasNext();
        } else {
          // Send all transactions in the same block
          while (txHasNext) {
            final Transaction tx = txs.next();
            if (tx != null) {
              String txHash =
                  besuNode.execute(ethTransactions.sendRawTransaction(tx.encoded().toHexString()));
              txHashes.add(txHash);
            }
            txHasNext = txs.hasNext();
          }
        }

        // After Paris, Clique as a consensus layer defined in the genesis file
        // doesn't work anymore
        // We use EngineAPIService to mimick the consensus layer steps and build a new block
        Block blockInfo = this.besuNode.execute(ethTransactions.block());
        nextFork = nextBlockFork(blockInfo);
        callEngineAPIToBuildNewBlock(besuNode, ethTransactions, genesisConfigBuilder, nextFork);

        // We check that the transactions are included in a block
        waitForTxReceipts(besuNode, ethTransactions, txHashes, txReceiptProcessed, blockNumbers);
        currentFork = nextFork;

        // We trace the conflation
        // For now conflations are composed of a single block triggered by
        // callEngineAPIToBuildNewBlock
        Block blockInfoUpdated = this.besuNode.execute(ethTransactions.block());
        checkState(blockNumbers.isEmpty() == blockInfoUpdated.getTransactions().isEmpty());
        if (blockNumbers.isEmpty()) {
          // If the block has no transactions, we retrieve the block numbers to trace from the last
          // traced until the latest block
          firstBlockNumber = finalBlockNumber + 1;
          finalBlockNumber = blockInfoUpdated.getNumber().longValue();
        } else {
          firstBlockNumber = Collections.min(blockNumbers);
          finalBlockNumber = Collections.max(blockNumbers);
        }
        TraceFile traceFile = traceAndCheckTracer(firstBlockNumber, finalBlockNumber, currentFork);
        Path traceFilePath = Path.of(traceFile.conflatedTracesFileName());

        // Clean up for next transaction
        resetTxReceipts(txReceiptProcessed);
        resetBlockNumbers(blockNumbers);
        resetTxHashes(txHashes);

        // Execution proof request
        requestAndStoreExecutionProof(
            besuNode,
            ethTransactions,
            firstBlockNumber,
            finalBlockNumber,
            traceFile,
            traceFilePath,
            testDataDir);
      }
    } catch (IOException | InterruptedException e) {
      throw new RuntimeException(e);
    } finally {
      try {
        besuCluster.close();
      } catch (Exception e) {
        log.error("Error closing besu cluster: %s".formatted(e.getMessage()), e);
      }

      try {
        shomeiNode.close();
      } catch (Exception e) {
        log.error("Error closing shomei node: %s".formatted(e.getMessage()), e);
      }
      try {
        besuNode.close();
      } catch (Exception e) {
        log.error("Error closing besu node: %s".formatted(e.getMessage()), e);
      }
    }
  }

  /// // /////////////////////////
  /// RPC Calls Helper Section
  /// // /////////////////////////

  private TraceFile lineaGenerateConflatedTracesToFileV2(
      final long startBlockNumber, final long endBlockNumber) throws IOException {
    return jsonRpcRequest(
        besuNode.jsonRpcBaseUrl().get(),
        "linea_generateConflatedTracesToFileV2",
        new TraceRequestParams(startBlockNumber, endBlockNumber, "test"),
        TraceFile.class);
  }

  private MerkelProofResponse rollupGetZkEVMStateMerkleProofV0(
      final long startBlockNumber, final long endBlockNumber) throws IOException {
    return jsonRpcRequest(
        shomeiNode.getJsonRpcUrl(),
        "rollup_getZkEVMStateMerkleProofV0",
        new RollupGetZkEvmStateV0Parameter(
            String.valueOf(startBlockNumber), String.valueOf(endBlockNumber), "test"),
        MerkelProofResponse.class);
  }

  private <R, P> R jsonRpcRequest(
      final String jsonRpcUrl, final String method, final P params, final Class<R> responseType)
      throws IOException {
    String request = createJsonRpcRequest(method, params, "1");
    Response response =
        httpClient
            .newCall(
                new Request.Builder()
                    .url(jsonRpcUrl)
                    .post(RequestBody.create(request, MEDIA_TYPE_JSON))
                    .build())
            .execute();
    String responseBody = response.body().string();
    assertThat(response.isSuccessful())
        .withFailMessage(
            String.format(
                "Unexpected response code: %s, body: %s, request: %s",
                response.code(), responseBody, request))
        .isTrue();
    JsonNode jsonRpcResponse = CONVERTER.fromJson(responseBody, JsonNode.class);
    JsonNode result = jsonRpcResponse.get("result");
    assertThat(result)
        .withFailMessage(
            String.format(
                "Request failed. response code: %s, body: %s, request: %s",
                response.code(), responseBody, request))
        .isNotNull();
    return parseJsonRpcResult(result, responseType);
  }

  private static <P> String createJsonRpcRequest(
      final String method, final P params, final String id) {
    return String.format(
        "{\n"
            + "    \"jsonrpc\": \"2.0\",\n"
            + "    \"method\": \"%s\",\n"
            + "    \"params\": [%s],\n"
            + "    \"id\": %s\n"
            + "}",
        method, CONVERTER.toJson(params), id);
  }

  @SuppressWarnings("unchecked")
  private static <R> R parseJsonRpcResult(final JsonNode result, final Class<R> responseType)
      throws JsonProcessingException {
    if (responseType.equals(MerkelProofResponse.class)) {
      return (R)
          new MerkelProofResponse(
              result.get("zkParentStateRootHash").asText(),
              result.get("zkEndStateRootHash").asText(),
              (ArrayNode) result.get("zkStateMerkleProof"),
              "test");
    } else {
      return MAPPER.treeToValue(result, responseType);
    }
  }

  private static int findFreePort() {
    int port = 0;
    try (ServerSocket socket = new ServerSocket(0)) {
      socket.setReuseAddress(true);
      port = socket.getLocalPort();
    } catch (IOException ignored) {
    }
    if (port > 0) {
      return port;
    }
    throw new RuntimeException("Could not find a free port");
  }

  /// // /////////////////////////
  /// Dynamic parsing of Genesis file
  /// // /////////////////////////

  private Fork nextBlockFork(Block block) {
    var nextTotalDifficulty =
        block.getTotalDifficulty().add(BigInteger.TWO); /* Clique increments by 2 */
    var nextBlockTimestamp = block.getTimestamp().longValue() + 1L;

    var TTD = genesisConfigBuilder.getTTD();
    var shanghaiTime = genesisConfigBuilder.getShanghaiTime();
    var cancunTime = genesisConfigBuilder.getCancunTime();
    var pragueTime = genesisConfigBuilder.getPragueTime();
    var osakaTime = genesisConfigBuilder.getOsakaTime();

    // No fork switch specified in the genesis file, stay in London
    if (TTD == null && shanghaiTime == null && cancunTime == null && pragueTime == null) {
      return Fork.LONDON;
    }

    var terminalTotalDifficulty = new BigInteger(TTD);

    // Fork from Paris specified
    if (nextTotalDifficulty.compareTo(terminalTotalDifficulty) > 0) {
      if (shanghaiTime != null && (nextBlockTimestamp >= parseLong(shanghaiTime))) {
        if (cancunTime != null && (nextBlockTimestamp >= parseLong(cancunTime))) {
          if (pragueTime != null && (nextBlockTimestamp >= parseLong(pragueTime))) {
            if (osakaTime != null && (nextBlockTimestamp >= parseLong(osakaTime))) {
              return OSAKA;
            }
            return Fork.PRAGUE;
          }
          return Fork.CANCUN;
        }
        return Fork.SHANGHAI;
      }
      return Fork.PARIS;
    }
    return Fork.LONDON;
  }

  /// // /////////////////////////
  /// Cleaning helpers
  /// // /////////////////////////

  private void resetTxReceipts(Map<String, Boolean> txReceiptProcessed) {
    txReceiptProcessed.clear();
  }

  private void resetBlockNumbers(ConcurrentSet<Long> blockNumbers) {
    blockNumbers.clear();
  }

  private void resetTxHashes(List<String> txHashes) {
    txHashes.clear();
  }

  /// // /////////////////////////
  /// Block building helpers
  /// // /////////////////////////

  private static void callEngineAPIToBuildNewBlock(
      BesuNode besuNode,
      EthTransactions ethTransactions,
      GenesisConfigBuilder genesisConfigBuilder,
      Fork nextFork)
      throws IOException, InterruptedException {
    ObjectMapper mapper = new ObjectMapper();
    EngineAPIService engineApiService = new EngineAPIService(besuNode, ethTransactions, mapper);
    BigInteger latestTimestamp = besuNode.execute(ethTransactions.block()).getTimestamp();
    long blockBuildingTimeMs =
        parseLong(genesisConfigBuilder.getCliqueBlockPeriodSeconds()) * 10000;
    engineApiService.buildNewBlock(nextFork, latestTimestamp.longValue() + 1L, blockBuildingTimeMs);
  }

  private void waitForTxReceipts(
      BesuNode besuNode,
      EthTransactions ethTransactions,
      List<String> txHashes,
      Map<String, Boolean> txReceiptProcessed,
      ConcurrentSet<Long> blockNumbers) {
    waitFor(
        100,
        () -> {
          txHashes.forEach(
              (hash) -> {
                if (txReceiptProcessed.containsKey(hash)) {
                  return;
                }
                var maybeTxReceipt = besuNode.execute(ethTransactions.getTransactionReceipt(hash));
                assertThat(maybeTxReceipt).isPresent();
                var txReceipt = maybeTxReceipt.get();
                blockNumbers.add(txReceipt.getBlockNumber().longValue());
                txReceiptProcessed.put(hash, true);
                log.info(
                    "Executed transaction txHash={}, blockNumber={}",
                    txReceipt.getTransactionHash(),
                    txReceipt.getBlockNumber());
              });
        });
  }

  /// // /////////////////////////
  /// Tracing helpers
  /// // /////////////////////////

  private static CorsetValidator getCorsetValidatorPerFork(Fork fork) {
    return new CorsetValidator(ChainConfig.MAINNET_TESTCONFIG(fork));
  }

  private TraceFile traceAndCheckTracer(long startBlockNumber, long endBlockNumber, Fork nextFork)
      throws IOException {
    TraceFile traceFile = lineaGenerateConflatedTracesToFileV2(startBlockNumber, endBlockNumber);
    Path traceFilePath = Path.of(traceFile.conflatedTracesFileName());
    waitFor(
        60,
        () -> {
          assertThat(traceFilePath.toFile().exists())
              .withFailMessage("Trace file %s does not exist", traceFilePath)
              .isTrue();
        });

    ExecutionEnvironment.checkTracer(
        traceFilePath, getCorsetValidatorPerFork(nextFork), false, Optional.of(log));

    return traceFile;
  }

  /// // /////////////////////////
  /// Proof helpers
  /// // /////////////////////////

  private void requestAndStoreExecutionProof(
      BesuNode besuNode,
      EthTransactions ethTransactions,
      long startBlockNumber,
      long endBlockNumber,
      TraceFile traceFile,
      Path traceFilePath,
      Path testDataDir)
      throws IOException {
    String previousBlockStateRootShanghai =
        besuNode
            .execute(
                ethTransactions.block(
                    DefaultBlockParameter.valueOf(BigInteger.valueOf(startBlockNumber - 1))))
            .getStateRoot();
    MerkelProofResponse merkelProofResponse =
        rollupGetZkEVMStateMerkleProofV0(startBlockNumber, endBlockNumber);
    log.info("rollupGetZkEVMStateMerkleProofV0={}", merkelProofResponse);
    ExecutionProof.BatchExecutionProofRequestDto executionProofRequestDto =
        new ExecutionProof.BatchExecutionProofRequestDto(
            merkelProofResponse.zkParentStateRootHash(),
            previousBlockStateRootShanghai,
            traceFilePath.getFileName().toString(),
            traceFile.tracesEngineVersion(),
            merkelProofResponse.zkStateManagerVersion(),
            merkelProofResponse.zkStateMerkleProof(),
            Collections.emptyList() /* blocksData */);

    String executionProofFileName =
        ExecutionProof.getExecutionProofRequestFilename(
            startBlockNumber,
            endBlockNumber,
            traceFile.tracesEngineVersion(),
            merkelProofResponse.zkStateManagerVersion());
    File executionProofRequestFile = testDataDir.resolve(executionProofFileName).toFile();
    assertThat(executionProofRequestFile.createNewFile()).isTrue();
    MAPPER.writeValue(executionProofRequestFile, executionProofRequestDto);
  }
}
