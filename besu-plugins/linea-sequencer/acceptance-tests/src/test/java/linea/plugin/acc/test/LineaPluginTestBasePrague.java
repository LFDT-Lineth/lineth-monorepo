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

package linea.plugin.acc.test;

import java.io.IOException;
import java.util.Collection;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.consensus.clique.CliqueExtraData;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.ethereum.eth.transactions.ImmutableTransactionPoolConfiguration;
import org.hyperledger.besu.ethereum.eth.transactions.TransactionPoolConfiguration;
import org.hyperledger.besu.tests.acceptance.dsl.EngineAPIService;
import org.hyperledger.besu.tests.acceptance.dsl.node.RunnableNode;
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory;
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory.CliqueOptions;
import org.junit.jupiter.api.BeforeEach;

// This file initializes a Besu node configured for the Prague fork and makes it available to
// acceptance tests.
@Slf4j
public abstract class LineaPluginTestBasePrague extends LineaPluginTestBase {
  private EngineAPIService engineApiService;
  private ObjectMapper mapper;
  private final String GENESIS_FILE_TEMPLATE_PATH = "/clique/clique-prague.json.tpl";

  @BeforeEach
  @Override
  public void setup() throws Exception {
    minerNode =
        createCliqueNodeWithExtraCliOptionsAndRpcApis(
            "miner1", getCliqueOptions(), getTestCliOptions(), Set.of("LINEA", "MINER"), true);
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
        GenesisConfigurationFactory.readGenesisFile(GENESIS_FILE_TEMPLATE_PATH);
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
}
