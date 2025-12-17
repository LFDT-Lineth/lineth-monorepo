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

package net.consensys.linea.zktracer;

import static net.consensys.linea.zktracer.Trace.*;

import net.consensys.linea.plugins.BesuServiceProvider;
import org.hyperledger.besu.datatypes.HardforkId;
import org.hyperledger.besu.datatypes.HardforkId.MainnetHardforkId;
import org.hyperledger.besu.evm.gascalculator.*;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.BlockchainService;

/**
 * release numbers of forks are defined from the <b>Ethereum Protocol Releases</b> table in <a
 * href="https://github.com/ethereum/execution-specs">execution specs</a> repo. We start counting at
 * 1 and include all named releases, including aborted ones (such as "DAO Wars").
 */
public enum Fork {
  LONDON(EVM_LONDON),
  PARIS(EVM_PARIS),
  SHANGHAI(EVM_SHANGHAI),
  CANCUN(EVM_CANCUN),
  PRAGUE(EVM_PRAGUE),
  OSAKA(EVM_OSAKA), // live on L1&L2 2025/12/03
  AMSTERDAM(EVM_AMSTERDAM) // for Q? 2026
;
  private final int releaseNumber;

  Fork(int releaseNumber) {
    this.releaseNumber = releaseNumber;
  }

  public int getReleaseNumber() {
    return releaseNumber;
  }

  public static String toCamelCase(Fork fork) {
    return switch (fork) {
      case OSAKA -> "osaka";
      // case AMSTERDAM -> "amsterdam";
      case LONDON, PARIS, SHANGHAI, CANCUN, PRAGUE ->
          throw new IllegalArgumentException("Fork no more supported by the tracer: " + fork);
      default -> throw new IllegalArgumentException("Unknown fork: " + fork);
    };
  }

  /**
   * Construct a fork instance from the name of a fork (e.g. "London", "Shanghai", etc). Observe
   * that case does not matter here. Hence, "LONDON", "London", "london", "lonDon" are all suitable
   * aliases for the LONDON instance.
   *
   * @param fork
   * @return
   */
  public static Fork fromString(String fork) {
    return Fork.valueOf(fork.toUpperCase());
  }

  private static boolean forkIsAtLeast(Fork fork, Fork threshold) {
    return fork.getReleaseNumber() >= threshold.getReleaseNumber();
  }

  public static boolean isPostShanghai(Fork fork) {
    return forkIsAtLeast(fork, SHANGHAI);
  }

  public static boolean isPostCancun(Fork fork) {
    return forkIsAtLeast(fork, CANCUN);
  }

  public static boolean isPostPrague(Fork fork) {
    return forkIsAtLeast(fork, PRAGUE);
  }

  public static boolean isPostOsaka(Fork fork) {
    return forkIsAtLeast(fork, OSAKA);
  }

  public static boolean isPostAmsterdam(Fork fork) {
    return forkIsAtLeast(fork, AMSTERDAM);
  }

  private static boolean forkPredates(Fork fork, Fork threshold) {
    return !forkIsAtLeast(fork, threshold);
  }

  public static boolean forkPredatesOsaka(Fork fork) {
    return forkPredates(fork, OSAKA);
  }

  public static boolean forkPredatesAmsterdam(Fork fork) {
    return forkPredates(fork, AMSTERDAM);
  }

  /**
   * Map MainnetHardforkId, datatype from Besu, to Fork enum instance
   *
   * @param hardForkId the hardfork id retrieved from Besu API
   * @return Fork
   */
  public static Fork fromMainnetHardforkIdToTracerFork(MainnetHardforkId hardForkId) {
    // equivalent to return Fork.valueOf(hardForkId.name());
    return switch (hardForkId) {
      case MainnetHardforkId.OSAKA -> OSAKA;
      // case MainnetHardforkId.AMSTERDAM -> AMSTERDAM;
      case MainnetHardforkId.LONDON,
          MainnetHardforkId.PARIS,
          MainnetHardforkId.SHANGHAI,
          MainnetHardforkId.CANCUN,
          MainnetHardforkId.PRAGUE ->
          throw new IllegalArgumentException("Fork no more supported by the tracer: " + hardForkId);
      default -> throw new IllegalArgumentException("Unknown fork: " + hardForkId);
    };
  }

  /**
   * Start a Besu Blockchain service and retrieve the hardfork id for a given block range
   *
   * @param fromBlock the block number at which to retrieve the hardfork id
   * @param toBlock the block number at which to retrieve the hardfork id
   * @return Fork corresponding Fork instance if the hardfork id is the same between fromBlock and
   *     toBlock, else throw
   */
  public static Fork getForkFromBesuBlockchainService(
      BlockchainService blockchainService, long fromBlock, long toBlock) {
    final HardforkId forkStart = blockchainService.getHardforkId(fromBlock);
    if (fromBlock != toBlock) {
      final HardforkId forkEnd = blockchainService.getHardforkId(toBlock);
      // Do not accept conflations with different fork ...
      if (!forkStart.equals(forkEnd)) {
        throw new IllegalStateException(
            "Illegal fork change from  "
                + forkStart
                + " at start block "
                + fromBlock
                + " to "
                + forkEnd
                + " at end block "
                + toBlock);
      }
    }
    return fromMainnetHardforkIdToTracerFork((MainnetHardforkId) forkStart);
  }

  public static Fork getForkFromBesuBlockchainService(
      ServiceManager context, long blockNumber, long toBlock) {
    return getForkFromBesuBlockchainService(
        BesuServiceProvider.getBesuService(context, BlockchainService.class), blockNumber, toBlock);
  }

  /**
   * Start a Besu Blockchain service and retrieve the hardfork id for a given block number
   *
   * @param context the context on which to start the service
   * @param blockNumber the block number at which to retrieve the hardfork id
   * @return Fork corresponding Fork instance
   */
  public static Fork getForkFromBesuBlockchainService(ServiceManager context, long blockNumber) {
    return getForkFromBesuBlockchainService(context, blockNumber, blockNumber);
  }

  public static Trace getTraceFromFork(Fork fork) {
    return switch (fork) {
      case OSAKA -> new TraceOsaka();
      default -> throw new IllegalArgumentException("Unknown fork: " + fork);
    };
  }

  public static GasCalculator getGasCalculatorFromFork(Fork fork) {
    return switch (fork) {
      case OSAKA -> new OsakaGasCalculator();
      case LONDON, PARIS, SHANGHAI, CANCUN, PRAGUE ->
          throw new IllegalArgumentException("Fork no more supported by the tracer: " + fork);
      default -> throw new IllegalArgumentException("Unknown fork: " + fork);
    };
  }

  /**
   * Return the number of contract addresses seen by the system transaction during execution. This
   * is primary to testing purposes to ensure the right number were seen.
   *
   * @param fork
   * @return
   */
  public static int numberOfAddressesSeenBySystemTransaction(Fork fork) {
    return switch (fork) {
      case OSAKA -> 2;
      case LONDON, PARIS, SHANGHAI, CANCUN, PRAGUE ->
          throw new IllegalArgumentException("Fork no more supported by the tracer: " + fork);
      default -> throw new IllegalArgumentException("Unknown fork: " + fork);
    };
  }

  // Used for blockchain ref tests with the Paris exception of "Merge"
  public static String toPascalCase(Fork fork) {
    return switch (fork) {
      case OSAKA -> "Osaka";
      case LONDON, PARIS, SHANGHAI, CANCUN, PRAGUE ->
          throw new IllegalArgumentException("Fork no more supported by the tracer: " + fork);
      default -> throw new IllegalArgumentException("Unknown fork: " + fork);
    };
  }
}
