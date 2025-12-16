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

package net.consensys.linea.plugins.rpc;

import static net.consensys.linea.zktracer.Fork.getForkFromBesuBlockchainService;
import static net.consensys.linea.zktracer.types.PublicInputs.generatePublicInputs;

import java.math.BigInteger;
import net.consensys.linea.plugins.BesuServiceProvider;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.plugins.config.LineaTracerSharedConfiguration;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.LineCountingTracer;
import net.consensys.linea.zktracer.ZkCounter;
import net.consensys.linea.zktracer.ZkTracer;
import net.consensys.linea.zktracer.types.PublicInputs;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.BlockchainService;

public class LineCounterGenerator {

  public static LineCountingTracer createLineCountingTracer(
      ServiceManager besuContext,
      LineaTracerSharedConfiguration tracerSharedConfiguration,
      LineaL1L2BridgeSharedConfiguration l1L2BridgeSharedConfiguration,
      long fromBlock,
      long toBlock) {
    final BlockchainService blockchainService =
        BesuServiceProvider.getBesuService(besuContext, BlockchainService.class);
    final Fork fork = getForkFromBesuBlockchainService(blockchainService, fromBlock, toBlock);

    if (tracerSharedConfiguration.isLimitless()) {
      return new ZkCounter(
          l1L2BridgeSharedConfiguration,
          fork,
          tracerSharedConfiguration.countHistoricalBlockHashes());
    } else {
      final BigInteger chainId =
          blockchainService
              .getChainId()
              .orElseThrow(() -> new IllegalStateException("ChainId must be provided"));
      if (tracerSharedConfiguration.countHistoricalBlockHashes()) {
        final PublicInputs publicInputs =
            generatePublicInputs(blockchainService, fromBlock, toBlock);
        return new ZkTracer(fork, l1L2BridgeSharedConfiguration, chainId, publicInputs);
      } else {
        return new ZkTracer(fork, l1L2BridgeSharedConfiguration, chainId);
      }
    }
  }
}
