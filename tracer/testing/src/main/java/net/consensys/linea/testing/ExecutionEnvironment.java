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

import static net.consensys.linea.testing.MultiBlockExecutionEnvironment.DEFAULT_DELTA_TIMESTAMP_BETWEEN_BLOCKS;
import static net.consensys.linea.testing.ToyExecutionEnvironmentV2.DEFAULT_TIME_STAMP;
import static net.consensys.linea.zktracer.Trace.*;
import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import java.lang.reflect.Method;
import java.lang.reflect.Parameter;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Optional;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.corset.CorsetValidator;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.ZkTracer;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.config.GenesisConfig;
import org.hyperledger.besu.config.GenesisConfigOptions;
import org.hyperledger.besu.consensus.clique.CliqueBlockHeaderFunctions;
import org.hyperledger.besu.consensus.clique.CliqueForksSchedulesFactory;
import org.hyperledger.besu.consensus.clique.CliqueProtocolSchedule;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SignatureAlgorithm;
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory;
import org.hyperledger.besu.cryptoservices.KeyPairSecurityModule;
import org.hyperledger.besu.cryptoservices.NodeKey;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.chain.BadBlockManager;
import org.hyperledger.besu.ethereum.core.BlockHeader;
import org.hyperledger.besu.ethereum.core.BlockHeaderBuilder;
import org.hyperledger.besu.ethereum.core.Difficulty;
import org.hyperledger.besu.ethereum.core.MiningConfiguration;
import org.hyperledger.besu.ethereum.mainnet.BalConfiguration;
import org.hyperledger.besu.ethereum.mainnet.MainnetProtocolSpecFactory;
import org.hyperledger.besu.ethereum.mainnet.ProtocolSchedule;
import org.hyperledger.besu.ethereum.mainnet.ProtocolSpec;
import org.hyperledger.besu.ethereum.mainnet.ProtocolSpecBuilder;
import org.hyperledger.besu.ethereum.mainnet.blockhash.CancunPreExecutionProcessor;
import org.hyperledger.besu.ethereum.mainnet.blockhash.PraguePreExecutionProcessor;
import org.hyperledger.besu.evm.internal.EvmConfiguration;
import org.hyperledger.besu.metrics.noop.NoOpMetricsSystem;
import org.junit.jupiter.api.TestInfo;
import org.slf4j.Logger;

@Slf4j
public class ExecutionEnvironment {
  public static final String CORSET_VALIDATION_RESULT = "Corset validation result: ";

  private static GenesisConfig getGenesisConfig(Fork fork) {
    return switch (fork) {
      case LONDON, PARIS, SHANGHAI, CANCUN ->
          GenesisConfig.fromSource(ExecutionEnvironment.class.getResource("/Linea_LONDON.json"));
      case PRAGUE, OSAKA ->
          GenesisConfig.fromSource(ExecutionEnvironment.class.getResource("/Linea_PRAGUE.json"));
      default -> throw new IllegalArgumentException("Unexpected fork value: " + fork);
    };
  }

  static final BlockHeaderBuilder DEFAULT_BLOCK_HEADER_BUILDER =
      BlockHeaderBuilder.createDefault()
          .number(ToyExecutionEnvironmentV2.DEFAULT_BLOCK_NUMBER)
          .timestamp(DEFAULT_TIME_STAMP)
          .parentHash(Hash.EMPTY_TRIE_HASH)
          .baseFee(ToyExecutionEnvironmentV2.DEFAULT_BASE_FEE)
          .nonce(0)
          .blockHeaderFunctions(new CliqueBlockHeaderFunctions());

  public static void checkTracer(
      Path traceFilePath,
      CorsetValidator corsetValidator,
      Boolean deleteTraceFile,
      Optional<Logger> logger) {
    boolean traceValidated = false;
    try {
      log.info("Corset checking the trace " + traceFilePath);
      CorsetValidator.Result corsetValidationResult = corsetValidator.validate(traceFilePath);
      traceValidated = corsetValidationResult.isValid();
      assertThat(traceValidated)
          .withFailMessage(CORSET_VALIDATION_RESULT + "%s", corsetValidationResult.corsetOutput())
          .isTrue();
    } finally {
      if (traceFilePath != null && traceValidated) {
        if (deleteTraceFile) {
          boolean traceFileDeleted = traceFilePath.toFile().delete();
          final Path finalTraceFilePath = traceFilePath;
          logger.ifPresent(
              log -> log.debug("trace file {} deleted {}", finalTraceFilePath, traceFileDeleted));
        }
      }
    }
  }

  public static void checkTracer(
      ZkTracer zkTracer,
      CorsetValidator corsetValidator,
      Optional<Logger> logger,
      long startBlock,
      long endBlock,
      TestInfo testInfo) {
    try {
      String prefix = constructTestPrefix(zkTracer.getChain(), testInfo);
      Path traceFilePath = Files.createTempFile(prefix, ".lt.gz");
      zkTracer.writeToFile(traceFilePath, startBlock, endBlock);
      final Path finalTraceFilePath = traceFilePath;
      logger.ifPresent(log -> log.debug("trace written to {}", finalTraceFilePath));
      checkTracer(
          traceFilePath,
          corsetValidator,
          !System.getenv().containsKey("PRESERVE_TRACE_FILES"),
          logger);
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
  }

  public static BlockHeaderBuilder getLineaBlockHeaderBuilder(
      Optional<BlockHeader> parentBlockHeader) {

    BlockHeaderBuilder blockHeaderBuilder =
        parentBlockHeader.isPresent()
            ? BlockHeaderBuilder.fromHeader(parentBlockHeader.get())
                .number(parentBlockHeader.get().getNumber() + 1)
                .timestamp(
                    parentBlockHeader.get().getTimestamp() + DEFAULT_DELTA_TIMESTAMP_BETWEEN_BLOCKS)
                .parentHash(parentBlockHeader.get().getHash())
                .nonce(parentBlockHeader.get().getNonce() + 1)
                .blockHeaderFunctions(new CliqueBlockHeaderFunctions())
            : DEFAULT_BLOCK_HEADER_BUILDER;

    return blockHeaderBuilder
        .baseFee(Wei.of(LINEA_BASE_FEE))
        // TODO: refacto this block gas limit
        .gasLimit(LINEA_BLOCK_GAS_LIMIT)
        .difficulty(Difficulty.of(LINEA_DIFFICULTY));
  }

  public static ProtocolSpec getProtocolSpec(BigInteger chainId, Fork fork) {
    final BadBlockManager badBlockManager = new BadBlockManager();
    final GenesisConfigOptions genesisConfigOptions = getGenesisConfig(fork).getConfigOptions();

    final ProtocolSchedule schedule =
        CliqueProtocolSchedule.create(
            genesisConfigOptions,
            CliqueForksSchedulesFactory.create(genesisConfigOptions),
            createNodeKey(),
            false,
            EvmConfiguration.DEFAULT,
            MiningConfiguration.MINING_DISABLED,
            badBlockManager,
            false,
            BalConfiguration.DEFAULT,
            new NoOpMetricsSystem());

    final MainnetProtocolSpecFactory protocol =
        new MainnetProtocolSpecFactory(
            Optional.of(chainId),
            true,
            genesisConfigOptions,
            EvmConfiguration.DEFAULT,
            MiningConfiguration.MINING_DISABLED,
            false,
            BalConfiguration.DEFAULT,
            new NoOpMetricsSystem());

    final ProtocolSpecBuilder builder =
        switch (fork) {
          case LONDON -> protocol.londonDefinition();
          case PARIS -> protocol.parisDefinition();
          case SHANGHAI -> protocol.shanghaiDefinition();
          case CANCUN ->
              protocol.cancunDefinition().preExecutionProcessor(new CancunPreExecutionProcessor());
          case PRAGUE ->
              protocol.pragueDefinition().preExecutionProcessor(new PraguePreExecutionProcessor());
          case OSAKA ->
              protocol.osakaDefinition().preExecutionProcessor(new PraguePreExecutionProcessor());
          default -> throw new IllegalArgumentException("Unexpected fork value: " + fork);
        };

    return builder.badBlocksManager(badBlockManager).build(schedule);
  }

  private static NodeKey createNodeKey() {
    final Bytes32 keyPairPrvKey =
        Bytes32.fromHexString("0xf7a58d5e755d51fa2f6206e91dd574597c73248aaf946ec1964b8c6268d6207b");
    final SignatureAlgorithm signatureAlgorithm = SignatureAlgorithmFactory.getInstance();
    final KeyPair keyPair =
        signatureAlgorithm.createKeyPair(signatureAlgorithm.createPrivateKey(keyPairPrvKey));
    final KeyPairSecurityModule keyPairSecurityModule = new KeyPairSecurityModule(keyPair);

    return new NodeKey(keyPairSecurityModule);
  }

  private static final String LINEA_PACKAGE = "net.consensys.linea.";

  /**
   * Construct a suitable prefix for the temporary lt file generated based on details (such as the
   * name) of the test in question. If a TestInfo instance is provided, this will be preferred. If
   * not, then it will fallback on a stack walking algorithm which attempts to figure out a suitable
   * prefix (which can fail, and never includes parametric test arguments).
   *
   * @return
   */
  public static String constructTestPrefix(ChainConfig chain, TestInfo testInfo) {
    String fork = chain.fork.toString();
    // Check whether we have suitable testInfo information.
    if (testInfo == null
        || testInfo.getTestMethod().isEmpty()
        || testInfo.getTestClass().isEmpty()) {
      // No, therefore fall back on legacy mechanism.  In principle, this should never happen now
      // and this could be
      // removed eventually.
      return constructLegacyTestPrefix(fork);
    }
    // Extract key information
    String className = testInfo.getTestClass().get().getSimpleName();
    Method method = testInfo.getTestMethod().get();
    String methodName = testInfo.getTestMethod().get().getName();
    String params = processTestName(method, testInfo.getDisplayName());
    // Done (for now)
    return (className + "_" + methodName + "_" + params + fork.toLowerCase() + "_")
        .replaceAll("…", "___");
    // Note that "…" is replaced with "___" as it is cannot be part of the filename
    // This is automatically added by JUnit when one of the arguments of a parametric test is too
    // long
  }

  /**
   * This method attempts to process the given test name into a more human-readable form. In
   * particular, for parameterised tests, we want to change things like "0x000001" into just "0x1",
   * etc.
   *
   * @param method the enclosing test method.
   * @param displayName provided display name to be processed.
   * @return
   */
  private static String processTestName(Method method, String displayName) {
    String[] values, split;
    StringBuilder builder = new StringBuilder();
    Parameter[] parameters = method.getParameters();
    int paramIndex = 0;
    // remove method name if it is embedded
    displayName = displayName.replace(method.getName(), "");
    // remove any commas
    displayName = displayName.replace(",", "");
    // split out any parameters
    split = displayName.split(" ");
    //
    for (int i = 0; i < split.length; i++) {
      String val = split[i];
      Parameter param;
      // Skip test index, as not super helpful.
      if (val.startsWith("[")) {
        continue;
      } else if (paramIndex < parameters.length
          && parameters[paramIndex].getType() == TestInfo.class) {
        continue;
      } else if (paramIndex < parameters.length) {
        builder.append(parameters[paramIndex].getName());
        builder.append("=");
      }
      //
      builder.append(processTestArgument(val));
      builder.append("_");
      paramIndex++;
    }
    // Limit maximum length of string to ensure the filename is not too long.
    String result = builder.toString();
    return result.substring(0, Math.min(100, result.length()));
  }

  private static String processTestArgument(String arg) {
    if (arg.isEmpty()) {
      return arg;
    } else if (Character.isDigit(arg.charAt(0))) {
      // Looks like a number
      return processTestNumericArgument(arg);
    } else {
      // default
      return arg;
    }
  }

  private static String processTestNumericArgument(String arg) {
    if (arg.startsWith("0x")) {
      // hex
      arg = arg.substring(2);
      arg = arg.replaceFirst("^0*", "");
      if (arg.isEmpty()) {
        return "0x0";
      }
      return "0x" + arg;
    } else {
      arg = arg.replaceFirst("^0*", "");
      if (arg.isEmpty()) {
        return "0";
      }
      return arg;
    }
  }

  /**
   * Construct a suitable prefix for the temporary lt file generated based on the method name of the
   * test. This is done by walking up the stack looking for a calling method whose classname ends
   * with "Test". Having found such a method, its name is then used as the test prefix. If no method
   * is found, then this simply returns null --- which is completely safe in this context.
   *
   * @return
   */
  public static String constructLegacyTestPrefix(String fork) {
    for (StackTraceElement ste : Thread.currentThread().getStackTrace()) {
      String className = ste.getClassName();
      if (className.endsWith("Test") || className.endsWith("Tests")) {
        // Yes, it is.  Now tidy up the name.
        String name = ste.getClassName().replace(LINEA_PACKAGE, "").replace(".", "_");
        // Done
        return name + "_" + ste.getMethodName() + "_" + fork.toLowerCase() + "_";
      }
    }
    // Failed, so return null.  This is fine as it just means the generate lt file will not have an
    // informative prefix.
    return fork;
  }
}
