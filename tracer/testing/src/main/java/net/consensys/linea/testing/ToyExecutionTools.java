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

import static net.consensys.linea.zktracer.Fork.*;
import static net.consensys.linea.zktracer.Trace.LINEA_BLOCK_GAS_LIMIT;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP2935HistoricalHash.EIP2935_HISTORY_STORAGE_ADDRESS;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP4788BeaconBlockRootSection.EIP4788_BEACONROOT_ADDRESS;
import static org.assertj.core.api.Assertions.assertThat;

import java.util.*;
import java.util.function.Consumer;
import lombok.SneakyThrows;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.corset.CorsetValidator;
import net.consensys.linea.zktracer.*;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.*;
import org.hyperledger.besu.ethereum.core.*;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.mainnet.MainnetTransactionProcessor;
import org.hyperledger.besu.ethereum.mainnet.ProtocolSpec;
import org.hyperledger.besu.ethereum.mainnet.TransactionValidationParams;
import org.hyperledger.besu.ethereum.mainnet.blockhash.PreExecutionProcessor;
import org.hyperledger.besu.ethereum.mainnet.systemcall.BlockProcessingContext;
import org.hyperledger.besu.ethereum.processing.TransactionProcessingResult;
import org.hyperledger.besu.ethereum.referencetests.GeneralStateTestCaseEipSpec;
import org.hyperledger.besu.ethereum.referencetests.ReferenceTestBlockchain;
import org.hyperledger.besu.ethereum.referencetests.ReferenceTestWorldState;
import org.hyperledger.besu.ethereum.rlp.RLP;
import org.hyperledger.besu.ethereum.vm.BlockchainBasedBlockHashLookup;
import org.hyperledger.besu.evm.EVM;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.fluent.SimpleBlockValues;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.worldstate.WorldUpdater;
import org.jetbrains.annotations.NotNull;
import org.junit.jupiter.api.TestInfo;

@Slf4j
public class ToyExecutionTools {
  private static final List<String> SPECS_PRIOR_TO_DELETING_EMPTY_ACCOUNTS =
      Arrays.asList("Frontier", "Homestead", "EIP150");

  private static final Bytes32 DEFAULT_PARENT_BEACON_BLOCK_ROOT =
      Bytes32.fromHexString("0x0011223344556677889900AABBCCDDEEFF0011223344556677889900AABBCCDD");

  private ToyExecutionTools() {
    // utility class
  }

  @SneakyThrows
  public static void executeTest(
      final GeneralStateTestCaseEipSpec spec,
      final ProtocolSpec protocolSpec,
      final LineCountingTracer tracer,
      final TransactionProcessingResultValidator transactionProcessingResultValidator,
      final Consumer<ZkTracer> zkTracerValidator,
      final TestInfo testInfo) {

    final BlockHeader blockHeader = getBlockHeader(spec, protocolSpec);

    final ReferenceTestWorldState initialWorldState = spec.getInitialWorldState();
    final List<Transaction> transactions = new ArrayList<>();
    for (int i = 0; i < spec.getTransactionsCount(); i++) {
      Transaction transaction = spec.getTransaction(i);

      // Sometimes the tests ask us assemble an invalid transaction.  If we have
      // no valid transaction then there is no test.  GeneralBlockChain tests
      // will handle the case where we receive the TXs in a serialized form.
      if (transaction == null) {
        assertThat(spec.getExpectException())
            .withFailMessage("Transaction was not assembled, but no exception was expected")
            .isNotNull();
        return;
      }

      transactions.add(transaction);
    }

    final BlockBody blockBody = new BlockBody(transactions, new ArrayList<>());
    final WorldUpdater worldStateUpdater = initialWorldState.updater();

    addSystemAccountsIfRequired(worldStateUpdater);

    final MainnetTransactionProcessor processor = protocolSpec.getTransactionProcessor();
    final ReferenceTestBlockchain blockchain = new ReferenceTestBlockchain(blockHeader.getNumber());
    final Wei blobGasPrice =
        protocolSpec
            .getFeeMarket()
            .blobGasPricePerGas(blockHeader.getExcessBlobGas().orElse(BlobGas.ZERO));

    tracer.traceStartConflation(1);
    tracer.traceStartBlock(worldStateUpdater, blockHeader, blockBody, blockHeader.getCoinbase());
    runSystemInitialTransactions(protocolSpec, initialWorldState, blockHeader, tracer);

    TransactionProcessingResult result = null;
    for (Transaction transaction : blockBody.getTransactions()) {
      // Several of the GeneralStateTests check if the transaction could potentially
      // consume more gas than is left for the block it's attempted to be included in.
      // This check is performed within the `BlockImporter` rather than inside the
      // `TransactionProcessor`, so these tests are skipped.
      if (transaction.getGasLimit() > blockHeader.getGasLimit() - blockHeader.getGasUsed()) {
        throw new IllegalArgumentException("Transaction gas limit higher that available in block");
      }

      result =
          processor.processTransaction(
              worldStateUpdater,
              blockHeader,
              transaction,
              blockHeader.getCoinbase(),
              tracer,
              new BlockchainBasedBlockHashLookup(blockHeader, blockchain),
              TransactionValidationParams.processingBlock(),
              blobGasPrice);

      if (result.isInvalid()) {
        final TransactionProcessingResult finalResult = result;
        assertThat(spec.getExpectException())
            .withFailMessage(() -> finalResult.getValidationResult().getErrorMessage())
            .isNotNull();
        return;
      }

      transactionProcessingResultValidator.accept(transaction, result);
      if (tracer instanceof ZkTracer) {
        zkTracerValidator.accept((ZkTracer) tracer);
      }
      worldStateUpdater.commit();
    }

    tracer.traceEndBlock(blockHeader, blockBody);
    tracer.traceEndConflation(worldStateUpdater);

    assertThat(spec.getExpectException())
        .withFailMessage("Exception was expected - " + spec.getExpectException())
        .isNull();

    final Account coinbase = worldStateUpdater.getOrCreate(spec.getBlockHeader().getCoinbase());
    if (coinbase != null && coinbase.isEmpty() && shouldClearEmptyAccounts(spec.getFork())) {
      worldStateUpdater.deleteAccount(coinbase.getAddress());
    }
    initialWorldState.persist(blockHeader);

    // Check the world state root hash.
    final Hash expectedRootHash = spec.getExpectedRootHash();
    Optional.ofNullable(expectedRootHash)
        .ifPresent(
            expected -> {
              assertThat(initialWorldState.rootHash())
                  .withFailMessage(
                      "Unexpected world state root hash; expected state: %s, computed state: %s",
                      spec.getExpectedRootHash(), initialWorldState.rootHash())
                  .isEqualTo(expected);
            });

    // Check the logs.
    final Hash expectedLogsHash = spec.getExpectedLogsHash();
    final TransactionProcessingResult finalResult = result;
    Optional.ofNullable(expectedLogsHash)
        .ifPresent(
            expected -> {
              assert finalResult != null;
              final List<Log> logs = finalResult.getLogs();

              assertThat(Hash.hash(RLP.encode(out -> out.writeList(logs, Log::writeTo))))
                  .withFailMessage("Unmatched logs hash. Generated logs: %s", logs)
                  .isEqualTo(expected);
            });

    if (tracer instanceof ZkTracer) {
      final ChainConfig chainConfig = ((ZkTracer) tracer).getChain();
      ExecutionEnvironment.checkTracer(
          (ZkTracer) tracer,
          new CorsetValidator(chainConfig),
          Optional.of(log),
          // block number for first block
          blockHeader.getNumber(),
          // block number for last block
          blockHeader.getNumber(),
          testInfo);
    }
  }

  /**
   * This method creates a block header from spec - with the only difference that it sets the parent
   * beacon block root to a default b-value if null
   */
  @NotNull
  private static BlockHeader getBlockHeader(
      GeneralStateTestCaseEipSpec spec, ProtocolSpec protocolSpec) {
    final BlockHeader specBlockHeader = spec.getBlockHeader();
    final BlockHeader blockHeader =
        new BlockHeader(
            specBlockHeader.getParentHash(),
            specBlockHeader.getOmmersHash(),
            specBlockHeader.getCoinbase(),
            specBlockHeader.getStateRoot(),
            specBlockHeader.getTransactionsRoot(),
            specBlockHeader.getReceiptsRoot(),
            specBlockHeader.getLogsBloom(),
            specBlockHeader.getDifficulty(),
            specBlockHeader.getNumber(),
            specBlockHeader.getGasLimit(),
            specBlockHeader.getGasUsed(),
            specBlockHeader.getTimestamp(),
            specBlockHeader.getExtraData(),
            specBlockHeader.getBaseFee().orElse(Wei.ONE),
            specBlockHeader.getMixHashOrPrevRandao(),
            specBlockHeader.getNonce(),
            specBlockHeader.getWithdrawalsRoot().orElse(Hash.ZERO),
            specBlockHeader.getBlobGasUsed().orElse(null),
            specBlockHeader.getExcessBlobGas().orElse(BlobGas.ZERO),
            specBlockHeader.getParentBeaconBlockRoot().orElse(DEFAULT_PARENT_BEACON_BLOCK_ROOT),
            specBlockHeader.getRequestsHash().orElse(Hash.ZERO),
            specBlockHeader.getBalHash().orElse(Hash.ZERO),
            protocolSpec.getBlockHeaderFunctions());
    return blockHeader;
  }

  public static void addSystemAccountsIfRequired(WorldUpdater worldStateUpdater) {
    if (worldStateUpdater.getAccount(EIP4788_BEACONROOT_ADDRESS) == null) {
      worldStateUpdater.createAccount(EIP4788_BEACONROOT_ADDRESS);
      // bytecode is taken from
      // https://etherscan.io/address/0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02#code
      worldStateUpdater
          .getAccount(EIP4788_BEACONROOT_ADDRESS)
          .setCode(
              Bytes.fromHexString(
                  "0x3373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500"));
    }
    if (worldStateUpdater.getAccount(EIP2935_HISTORY_STORAGE_ADDRESS) == null) {
      worldStateUpdater.createAccount(EIP2935_HISTORY_STORAGE_ADDRESS);
      // bytecode is taken from
      // https://etherscan.io/address/0x0000F90827F1C53a10cb7A02335B175320002935#code
      worldStateUpdater
          .getAccount(EIP2935_HISTORY_STORAGE_ADDRESS)
          .setCode(
              Bytes.fromHexString(
                  "0x3373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500"));
    }
    worldStateUpdater.commit();
  }

  @SneakyThrows
  public static long executeTestOnlyForGasCost(
      final GeneralStateTestCaseEipSpec spec,
      final ProtocolSpec protocolSpec,
      final ZkTracer tracer,
      final List<ToyAccount> accounts) {
    final MutableWorldState worldState = spec.getInitialWorldState();
    final WorldUpdater worldStateUpdater = worldState.updater();
    final MainnetTransactionProcessor processor = protocolSpec.getTransactionProcessor();
    final EVM evm = protocolSpec.getEvm();

    SimpleBlockValues blockValues = new SimpleBlockValues();
    blockValues.setBaseFee(Optional.of(Wei.of(1)));
    Account senderAccount = accounts.get(0);
    Account receiverAccount = accounts.get(1);
    Transaction tx = spec.getTransaction(0);
    Bytes txPayload = tx.getPayload();
    Wei txValue = tx.getValue();

    MessageFrame initialMessageFrame =
        MessageFrame.builder()
            .worldUpdater(worldStateUpdater)
            .gasPrice(Wei.ONE)
            .blobGasPrice(Wei.ONE)
            .blockValues(blockValues)
            .miningBeneficiary(Address.ZERO)
            .blockHashLookup((__, ___) -> Hash.ZERO)
            .completer(messageFrame -> {})
            .apparentValue(Wei.ZERO)
            .value(txValue)
            .inputData(txPayload)
            .originator(senderAccount.getAddress())
            .address(receiverAccount.getAddress())
            .contract(receiverAccount.getAddress())
            .sender(senderAccount.getAddress())
            // For gas cost purposes, we don't care about the Type of the message frame
            .type(MessageFrame.Type.MESSAGE_CALL)
            .initialGas(LINEA_BLOCK_GAS_LIMIT)
            .code(
                evm.getOrCreateCachedJumpDest(
                    receiverAccount.getCodeHash(), receiverAccount.getCode()))
            .build();

    Deque<MessageFrame> messageFrameStack = initialMessageFrame.getMessageFrameStack();
    while (!messageFrameStack.isEmpty()) {
      processor.process(messageFrameStack.peekFirst(), tracer);
    }

    final long intrinsicTxCostWithNoAccessOrDelegationCost =
        tracer.getHub().gasCalculator.transactionIntrinsicGasCost(tx, 0);

    return LINEA_BLOCK_GAS_LIMIT
        - initialMessageFrame.getRemainingGas()
        + intrinsicTxCostWithNoAccessOrDelegationCost;
  }

  private static boolean shouldClearEmptyAccounts(final String eip) {
    return !SPECS_PRIOR_TO_DELETING_EMPTY_ACCOUNTS.contains(eip);
  }

  public static void runSystemInitialTransactions(
      ProtocolSpec spec,
      MutableWorldState world,
      BlockHeader header,
      ConflationAwareOperationTracer tracer) {
    final PreExecutionProcessor preExecutionProcessor = spec.getPreExecutionProcessor();
    final ReferenceTestBlockchain blockchain = new ReferenceTestBlockchain(header.getNumber());

    final BlockProcessingContext context =
        new BlockProcessingContext(
            header,
            world,
            spec,
            preExecutionProcessor.createBlockHashLookup(blockchain, header),
            tracer,
            Optional.empty());
    preExecutionProcessor.process(context, Optional.empty());
  }
}
