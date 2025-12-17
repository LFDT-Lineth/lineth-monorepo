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
import static net.consensys.linea.reporting.TracerTestBase.chainConfig;
import static net.consensys.linea.reporting.TracerTestBase.fork;
import static net.consensys.linea.zktracer.ChainConfig.MAINNET_TESTCONFIG;
import static net.consensys.linea.zktracer.Fork.*;
import static net.consensys.linea.zktracer.Trace.LINEA_BASE_FEE;
import static net.consensys.linea.zktracer.types.PublicInputs.getDefaultBlobBaseFees;

import java.util.*;
import java.util.function.Consumer;
import java.util.function.Supplier;
import java.util.stream.Collectors;
import lombok.Builder;
import lombok.Getter;
import lombok.Setter;
import lombok.Singular;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.ZkCounter;
import net.consensys.linea.zktracer.ZkTracer;
import net.consensys.linea.zktracer.module.hub.Hub;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.*;
import org.hyperledger.besu.ethereum.core.*;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.mainnet.ProtocolSpec;
import org.hyperledger.besu.ethereum.referencetests.GeneralStateTestCaseEipSpec;
import org.hyperledger.besu.ethereum.referencetests.ReferenceTestWorldState;
import org.junit.jupiter.api.TestInfo;

@Builder
@Slf4j
public class ToyExecutionEnvironmentV2 {
  @Builder.Default public final ChainConfig unitTestsChain = MAINNET_TESTCONFIG(OSAKA);
  public final TestInfo testInfo;
  public static final Address DEFAULT_COINBASE_ADDRESS =
      Address.fromHexString("0xc019ba5e00000000c019ba5e00000000c019ba5e");
  public static final long DEFAULT_BLOCK_NUMBER = 6678980;
  public static final Map<Long, Bytes> DEFAULT_BLOB_BASE_FEES =
      getDefaultBlobBaseFees(DEFAULT_BLOCK_NUMBER, DEFAULT_BLOCK_NUMBER);
  public static final long DEFAULT_TIME_STAMP = 1347310;
  public static final Hash DEFAULT_HASH =
      Hash.fromHexStringLenient("0xdeadbeef123123666dead666dead666");
  public static final Bytes32 DEFAULT_BEACON_ROOT = Bytes32.fromHexStringLenient("cc".repeat(32));
  public static final Wei DEFAULT_BASE_FEE = Wei.of(LINEA_BASE_FEE);

  @Builder.Default private final List<ToyAccount> accounts = Collections.emptyList();
  @Builder.Default private final Address coinbase = DEFAULT_COINBASE_ADDRESS;
  @Builder.Default private final Boolean runWithBesuNode = false;
  @Builder.Default private String customBesuNodeGenesis = null;
  @Builder.Default private Boolean oneTxPerBlockOnBesuNode = false;
  @Builder.Default private final long firstBlockNumber = DEFAULT_BLOCK_NUMBER;
  @Builder.Default private static final Map<Long, Bytes> blobBaseFees = DEFAULT_BLOB_BASE_FEES;

  @Singular private final List<Transaction> transactions;

  /**
   * A transaction validator of each transaction; by default, it asserts that the transaction was
   * successfully processed.
   */
  @Builder.Default
  private final TransactionProcessingResultValidator transactionProcessingResultValidator =
      TransactionProcessingResultValidator.EMPTY_VALIDATOR;

  // This was previously DEFAULT_VALIDATOR, however some tests we write are supposed to generate
  // failing transactions
  // Thus we cannot use the DEFAULT_VALIDATOR since it asserts that the transaction is successful

  @Builder.Default private final Consumer<ZkTracer> zkTracerValidator = x -> {};

  ZkTracer tracer;
  @Setter @Getter public ZkCounter zkCounter;

  public static ToyExecutionEnvironmentV2.ToyExecutionEnvironmentV2Builder builder(
      ChainConfig chainConfig, TestInfo testInfo) {
    return new ToyExecutionEnvironmentV2Builder()
        .unitTestsChain(chainConfig)
        .testInfo(testInfo)
        .tracer(new ZkTracer(chainConfig, blobBaseFees));
  }

  public void run() {
    if (runWithBesuNode || System.getenv().containsKey("RUN_WITH_BESU_NODE")) {
      BesuExecutionTools besuExecTools =
          new BesuExecutionTools(
              Optional.of(testInfo),
              unitTestsChain,
              coinbase,
              accounts,
              transactions,
              oneTxPerBlockOnBesuNode,
              customBesuNodeGenesis);
      besuExecTools.executeTest();
    } else {
      final ProtocolSpec protocolSpec =
          ExecutionEnvironment.getProtocolSpec(unitTestsChain.id, unitTestsChain.fork);
      final GeneralStateTestCaseEipSpec generalStateTestCaseEipSpec =
          this.buildGeneralStateTestCaseSpec(protocolSpec);

      ToyExecutionTools.executeTest(
          generalStateTestCaseEipSpec,
          protocolSpec,
          tracer,
          transactionProcessingResultValidator,
          zkTracerValidator,
          testInfo);

      if (isPostOsaka(tracer.getHub().fork)) {
        // This is to check that the light counter is really counting more than the full tracer
        final ZkTracer tracer = this.tracer;

        final Map<String, Integer> tracerCount = tracer.getModulesLineCount();

        final ToyExecutionEnvironmentV2 copyEnvironment =
            ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
                .transactionProcessingResultValidator(
                    TransactionProcessingResultValidator.EMPTY_VALIDATOR)
                .accounts(accounts)
                .zkTracerValidator(zkTracerValidator)
                .transactions(transactions)
                .build();
        copyEnvironment.runForCounting();
        final Map<String, Integer> lightCounterCount =
            copyEnvironment.zkCounter.getModulesLineCount();

        final List<String> moduleToCheck =
            copyEnvironment.zkCounter.checkedModules().stream()
                .map(module -> module.moduleKey().toString())
                .toList();

        for (String module : moduleToCheck) {
          checkArgument(
              tracerCount.get(module) <= lightCounterCount.get(module),
              "Module "
                  + module
                  + " has more lines in full tracer: "
                  + tracerCount.get(module)
                  + " than in light counter: "
                  + lightCounterCount.get(module));

          // TODO: how to make it smart ?

          // Note: we compare to twice the (tracer count +1) to not get exceptions when tracer
          // module is empty (GAS for SKIP tx for example)
          // checkArgument(
          //     lightCounterCount.get(module) <= 2 * (tracerCount.get(module) + 1),
          //     "Module "
          //         + module
          //         + " has more than twice line counts in light tracer: "
          //         + lightCounterCount.get(module)
          //         + " than in full counter: "
          //         + tracerCount.get(module));
        }
      }
    }
  }

  public void runForCounting() {
    zkCounter = new ZkCounter(unitTestsChain.bridgeConfiguration, fork, true);

    final ProtocolSpec protocolSpec =
        ExecutionEnvironment.getProtocolSpec(unitTestsChain.id, unitTestsChain.fork);
    final GeneralStateTestCaseEipSpec generalStateTestCaseEipSpec =
        this.buildGeneralStateTestCaseSpec(protocolSpec);
    ToyExecutionTools.executeTest(
        generalStateTestCaseEipSpec,
        protocolSpec,
        zkCounter,
        transactionProcessingResultValidator,
        zkTracerValidator,
        testInfo);
  }

  public long runForGasCost() {
    final ProtocolSpec protocolSpec =
        ExecutionEnvironment.getProtocolSpec(unitTestsChain.id, unitTestsChain.fork);
    final GeneralStateTestCaseEipSpec generalStateTestCaseEipSpec =
        this.buildGeneralStateTestCaseSpec(protocolSpec);

    return ToyExecutionTools.executeTestOnlyForGasCost(
        generalStateTestCaseEipSpec, protocolSpec, tracer, this.accounts);
  }

  public Hub getHub() {
    if (runWithBesuNode || System.getenv().containsKey("RUN_WITH_BESU_NODE")) {
      throw new IllegalStateException("Cannot get Hub when running with Besu node");
    }
    return tracer.getHub();
  }

  public ZkTracer getZkTracer() {
    if (runWithBesuNode || System.getenv().containsKey("RUN_WITH_BESU_NODE")) {
      throw new IllegalStateException("Cannot get zkTracer when running with Besu node");
    }
    return tracer;
  }

  public GeneralStateTestCaseEipSpec buildGeneralStateTestCaseSpec(ProtocolSpec protocolSpec) {
    final Map<String, ReferenceTestWorldState.AccountMock> accountMockMap =
        accounts.stream()
            .collect(
                Collectors.toMap(
                    toyAccount -> toyAccount.getAddress().toHexString(),
                    ToyAccount::toAccountMock));
    final ReferenceTestWorldState referenceTestWorldState =
        ReferenceTestWorldState.create(accountMockMap, protocolSpec.getEvm().getEvmConfiguration());
    final BlockHeader blockHeader =
        ExecutionEnvironment.getLineaBlockHeaderBuilder(Optional.empty())
            .number(firstBlockNumber)
            .coinbase(coinbase)
            .timestamp(DEFAULT_TIME_STAMP)
            .parentHash(DEFAULT_HASH)
            .baseFee(DEFAULT_BASE_FEE)
            .buildBlockHeader();

    final List<Supplier<Transaction>> txSuppliers = new ArrayList<>();
    for (Transaction tx : transactions) {
      txSuppliers.add(() -> tx);
    }

    return new GeneralStateTestCaseEipSpec(
        /*fork*/ protocolSpec.getEvm().getEvmVersion().getName().toLowerCase(),
        txSuppliers,
        referenceTestWorldState,
        /*expectedRootHash*/ null,
        /*expectedLogsHash*/ null,
        blockHeader,
        /*dataIndex*/ -1,
        /*gasIndex*/ -1,
        /*valueIndex*/ -1,
        /*expectException*/ null);
  }
}
