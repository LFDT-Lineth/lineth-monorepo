/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.testing.ToyExecutionEnvironmentV2.*;
import static net.consensys.linea.zktracer.ChainConfig.MAINNET_TESTCONFIG;
import static net.consensys.linea.zktracer.Fork.*;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ExecutionEnvironment;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.ZkCounter;
import net.consensys.linea.zktracer.ZkTracer;
import org.hyperledger.besu.ethereum.core.BlockBody;
import org.hyperledger.besu.ethereum.core.BlockHeader;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.junit.jupiter.api.Test;

public class LineCountingTracerTest extends TracerTestBase {

  @Test
  void noDuplicateNamesInModules() {
    final ZkTracer tracer = new ZkTracer(chainConfig);
    final List<String> tracerToCount =
        tracer.getModulesToCount().stream().map(module -> module.moduleKey().toString()).toList();
    final List<String> tracedModules =
        tracer.getHub().getModulesToTrace().stream()
            .map(module -> module.moduleKey().toString())
            .toList();
    final List<String> refTables =
        tracer.getHub().refTableModules().stream()
            .map(module -> module.moduleKey().toString())
            .toList();

    // Check that all traced modules are counted or reference tables
    checkArgument(
        Stream.concat(tracerToCount.stream(), refTables.stream())
            .toList()
            .containsAll(tracedModules),
        "Some traced modules are not counted");

    // Search for duplicates
    checkArgument(
        tracerToCount.size() == tracerToCount.stream().distinct().toList().size(),
        "Duplicate has been found");
    final ZkCounter counter = new ZkCounter(chainConfig.bridgeConfiguration, fork);
    final List<String> counterToCount =
        counter.getModulesToCount().stream().map(module -> module.moduleKey().toString()).toList();
    checkArgument(
        counterToCount.size() == counterToCount.stream().distinct().toList().size(),
        "Duplicate has been found");
  }

  @Test
  void sameModuleAcrossAllForkWithZkCounterAndTracer() {
    final ZkCounter counter = new ZkCounter(chainConfig.bridgeConfiguration, fork);
    final List<String> counterModules =
        counter.getModulesToCount().stream().map(module -> module.moduleKey().toString()).toList();

    for (Fork fork : Fork.values()) {
      // The tracer doesn't support Prague and before now
      if (forkPredatesOsaka(fork)) {
        continue;
      }
      // TODO: reenable me when Amsterdam is supported
      if (isPostAmsterdam(fork)) {
        return;
      }
      final ChainConfig config = MAINNET_TESTCONFIG(fork);
      final ZkTracer tracer = new ZkTracer(config);
      final List<String> tracerModules =
          tracer.getModulesToCount().stream().map(module -> module.moduleKey().toString()).toList();

      // check that counter ⊆ tracer(fork)
      for (String module : counterModules) {
        checkArgument(
            tracerModules.contains(module),
            "Module " + module + " is missing in ZkTracer for fork " + fork);
      }
      // check that tracer(fork) ⊆ counter
      for (String module : tracerModules) {
        checkArgument(
            counterModules.contains(module),
            "Module " + module + " is missing in ZkCounter for fork " + fork);
      }
    }
  }

  @Test
  void startBlockStuffAreNotPopped() {
    final WorldView world = WorldView.EMPTY;
    final BlockHeader blockHeader =
        ExecutionEnvironment.getLineaBlockHeaderBuilder(Optional.empty())
            .number(DEFAULT_BLOCK_NUMBER)
            .coinbase(DEFAULT_COINBASE_ADDRESS)
            .timestamp(DEFAULT_TIME_STAMP)
            .parentHash(DEFAULT_HASH)
            .baseFee(DEFAULT_BASE_FEE)
            .parentBeaconBlockRoot(DEFAULT_BEACON_ROOT)
            .buildBlockHeader();
    final BlockBody blockBody = BlockBody.empty();

    final ZkTracer tracer = new ZkTracer(chainConfig);
    tracer.traceStartConflation(1);
    tracer.traceStartBlock(world, blockHeader, blockBody, DEFAULT_COINBASE_ADDRESS);
    final Map<String, Integer> sizeBeforeTracer = tracer.getModulesLineCount();
    tracer.popTransactionBundle();
    final Map<String, Integer> sizeAfterTracer = tracer.getModulesLineCount();
    for (String module : sizeBeforeTracer.keySet()) {
      checkArgument(
          Objects.equals(sizeAfterTracer.get(module), sizeBeforeTracer.get(module)),
          "Tracer: some block stuff has been removed in Module " + module);
    }

    final ZkCounter counter = new ZkCounter(chainConfig.bridgeConfiguration, fork);
    counter.traceStartConflation(1);
    counter.traceStartBlock(world, blockHeader, blockBody, DEFAULT_COINBASE_ADDRESS);
    final Map<String, Integer> sizeBeforeCounter = counter.getModulesLineCount();
    counter.popTransactionBundle();
    final Map<String, Integer> sizeAfterCounter = counter.getModulesLineCount();
    for (String module : sizeBeforeCounter.keySet()) {
      checkArgument(
          Objects.equals(sizeAfterCounter.get(module), sizeBeforeCounter.get(module)),
          "Counter: some block stuff has been removed in Module "
              + module
              + "line count is dropping from "
              + sizeBeforeCounter.get(module)
              + " to "
              + sizeAfterCounter.get(module));
    }
  }
}
