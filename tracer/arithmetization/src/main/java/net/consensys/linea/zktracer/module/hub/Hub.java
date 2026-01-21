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

package net.consensys.linea.zktracer.module.hub;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration.TEST_DEFAULT;
import static net.consensys.linea.zktracer.Fork.getGasCalculatorFromFork;
import static net.consensys.linea.zktracer.Trace.Hub.MULTIPLIER___STACK_STAMP;
import static net.consensys.linea.zktracer.module.ModuleName.*;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_EXEC;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_FINL;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_INIT;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_SKIP;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_WARM;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.*;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.*;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.types.AddressUtils.effectiveToAddress;
import static org.hyperledger.besu.evm.frame.MessageFrame.Type.*;

import java.util.*;
import java.util.stream.Stream;
import lombok.Getter;
import lombok.experimental.Accessors;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.CountingOnlyModule;
import net.consensys.linea.zktracer.container.module.IncrementAndDetectModule;
import net.consensys.linea.zktracer.container.module.IncrementingModule;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.bin.Bin;
import net.consensys.linea.zktracer.module.blake2f.Blake2f;
import net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeModexpData;
import net.consensys.linea.zktracer.module.blockdata.module.BlockData;
import net.consensys.linea.zktracer.module.blockdata.module.CancunBlockData;
import net.consensys.linea.zktracer.module.blockhash.Blockhash;
import net.consensys.linea.zktracer.module.blsdata.BlsData;
import net.consensys.linea.zktracer.module.ecdata.EcData;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.exp.Exp;
import net.consensys.linea.zktracer.module.ext.Ext;
import net.consensys.linea.zktracer.module.gas.Gas;
import net.consensys.linea.zktracer.module.hub.defer.DeferRegistry;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.StackFragment;
import net.consensys.linea.zktracer.module.hub.section.*;
import net.consensys.linea.zktracer.module.hub.section.CreateSection;
import net.consensys.linea.zktracer.module.hub.section.TxInitializationSection;
import net.consensys.linea.zktracer.module.hub.section.TxSkipSection;
import net.consensys.linea.zktracer.module.hub.section.call.CallSection;
import net.consensys.linea.zktracer.module.hub.section.copy.CallDataCopySection;
import net.consensys.linea.zktracer.module.hub.section.copy.CodeCopySection;
import net.consensys.linea.zktracer.module.hub.section.copy.ExtCodeCopySection;
import net.consensys.linea.zktracer.module.hub.section.copy.ReturnDataCopySection;
import net.consensys.linea.zktracer.module.hub.section.halt.ReturnSection;
import net.consensys.linea.zktracer.module.hub.section.halt.RevertSection;
import net.consensys.linea.zktracer.module.hub.section.halt.SelfdestructSection;
import net.consensys.linea.zktracer.module.hub.section.halt.StopSection;
import net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP2935HistoricalHash;
import net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP4788BeaconBlockRootSection;
import net.consensys.linea.zktracer.module.hub.section.systemTransaction.SysfNoopSection;
import net.consensys.linea.zktracer.module.hub.section.transients.TLoadSection;
import net.consensys.linea.zktracer.module.hub.section.transients.TStoreSection;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.module.hub.signals.PlatformController;
import net.consensys.linea.zktracer.module.hub.state.BlockStack;
import net.consensys.linea.zktracer.module.hub.state.State;
import net.consensys.linea.zktracer.module.hub.state.TransactionStack;
import net.consensys.linea.zktracer.module.hub.transients.Transients;
import net.consensys.linea.zktracer.module.limits.BlockTransactions;
import net.consensys.linea.zktracer.module.limits.Keccak;
import net.consensys.linea.zktracer.module.limits.L1BlockSize;
import net.consensys.linea.zktracer.module.limits.precompiles.BlakeRounds;
import net.consensys.linea.zktracer.module.limits.precompiles.RipemdBlocks;
import net.consensys.linea.zktracer.module.limits.precompiles.Sha256Blocks;
import net.consensys.linea.zktracer.module.logdata.LogData;
import net.consensys.linea.zktracer.module.loginfo.LogInfo;
import net.consensys.linea.zktracer.module.mmio.Mmio;
import net.consensys.linea.zktracer.module.mmu.Mmu;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.mul.Mul;
import net.consensys.linea.zktracer.module.mxp.Mxp;
import net.consensys.linea.zktracer.module.oob.Oob;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlpaddr.RlpAddr;
import net.consensys.linea.zktracer.module.rlptxn.RlpTxn;
import net.consensys.linea.zktracer.module.rlptxrcpt.RlpTxnRcpt;
import net.consensys.linea.zktracer.module.rom.Rom;
import net.consensys.linea.zktracer.module.romlex.ContractMetadata;
import net.consensys.linea.zktracer.module.romlex.RomLex;
import net.consensys.linea.zktracer.module.shakiradata.ShakiraData;
import net.consensys.linea.zktracer.module.shf.Shf;
import net.consensys.linea.zktracer.module.stp.Stp;
import net.consensys.linea.zktracer.module.tables.BlsRt;
import net.consensys.linea.zktracer.module.tables.InstructionDecoder;
import net.consensys.linea.zktracer.module.trm.Trm;
import net.consensys.linea.zktracer.module.txndata.TxnData;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.OpCodes;
import net.consensys.linea.zktracer.opcode.gas.projector.GasProjector;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.runtime.callstack.CallFrameType;
import net.consensys.linea.zktracer.runtime.callstack.CallStack;
import net.consensys.linea.zktracer.runtime.stack.StackContext;
import net.consensys.linea.zktracer.runtime.stack.StackLine;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.MemoryRange;
import net.consensys.linea.zktracer.types.PublicInputs;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.account.AccountState;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.gascalculator.GasCalculator;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.log.LogTopic;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

@Slf4j
@Accessors(fluent = true)
@Getter
public final class Hub implements Module {
  /** Active fork for this hub. */
  public final Fork fork;

  /** Active opcode information for this hub. */
  private final OpCodes opCodes;

  /** The {@link GasCalculator} used in this version of the arithmetization */
  public final GasCalculator gasCalculator;

  public final GasProjector gasProjector;

  /** accumulate the trace information for the Hub */
  public final State state = new State();

  /** contain the factories for trace segments that need complex initialization */
  private final Factories factories = new Factories(this);

  /** provides phase-related volatile information */
  Transients transients = new Transients(this);

  /**
   * Long-lived states, not used in tracing per se but keeping track of data of the associated
   * lifetime
   */
  CallStack callStack = new CallStack();

  /** Stores the transaction Metadata of all the transaction of the conflated block */
  TransactionStack txStack = new TransactionStack();

  /** Stores the block Metadata of all the blocks of the conflation */
  BlockStack blockStack = new BlockStack();

  /** Stores all the actions that must be deferred to a later time */
  private final DeferRegistry defers = new DeferRegistry();

  /** stores all data related to failure states & module activation */
  private final PlatformController pch = new PlatformController(this);

  @Override
  public ModuleName moduleKey() {
    return HUB;
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.hub().headers(this.lineCount());
  }

  @Override
  public void commit(Trace trace) {
    state.commit(trace.hub());
  }

  @Override
  public int lineCount() {
    return state.lineCounter().lineCount();
  }

  @Override
  public int spillage(Trace trace) {
    return trace.hub().spillage();
  }

  // List of all modules of the ZK-evm:

  // stateless modules
  private final Wcp wcp = new Wcp();
  private final Add add = new Add();
  private final Bin bin = new Bin();
  private final Blockhash blockhash;
  private final Euc euc = new Euc();
  private final Ext ext = new Ext();
  private final Gas gas = new Gas();
  private final Mul mul = new Mul();
  private final Mod mod = new Mod();
  private final Shf shf = new Shf();
  private final Trm trm;
  private final RlpUtils rlpUtils = new RlpUtils();

  // other
  private final BlockData blockdata;
  private final RomLex romLex = new RomLex(this);
  private final Rom rom = new Rom(romLex);
  private final RlpTxn rlpTxn;
  private final Mmio mmio;
  @Getter final TxnData txnData = new TxnData(this, wcp, euc);
  private final RlpTxnRcpt rlpTxnRcpt = new RlpTxnRcpt();
  private final LogInfo logInfo = new LogInfo(rlpTxnRcpt);
  private final LogData logData = new LogData(rlpTxnRcpt);
  private final RlpAddr rlpAddr;

  // modules triggered by sub-fragments of the MISCELLANEOUS / IMC perspective
  private final Mxp mxp = new Mxp();
  private final Oob oob = new Oob();
  private final Mmu mmu;
  private final Stp stp = new Stp();
  private final Exp exp = new Exp();

  /*
   * Those modules are not traced, we just compute the number of calls to those
   * precompile to meet the prover limits
   */
  private final BlockTransactions blockTransactions = new BlockTransactions();
  private final Keccak keccak;
  private final Sha256Blocks sha256Blocks = new Sha256Blocks();

  // related to EcData
  private final IncrementingModule ecAddEffectiveCall =
      new IncrementingModule(PRECOMPILE_ECADD_EFFECTIVE_CALLS);
  private final IncrementingModule ecMulEffectiveCall =
      new IncrementingModule(PRECOMPILE_ECMUL_EFFECTIVE_CALLS);
  private final IncrementingModule ecRecoverEffectiveCall =
      new IncrementingModule(PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS);
  private final CountingOnlyModule ecPairingG2MembershipCalls =
      new CountingOnlyModule(PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS);
  private final CountingOnlyModule ecPairingMillerLoops =
      new CountingOnlyModule(PRECOMPILE_ECPAIRING_MILLER_LOOPS);
  private final IncrementingModule ecPairingFinalExponentiations =
      new IncrementingModule(PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS);
  private final IncrementingModule p256VerifyEffectiveCalls =
      new IncrementingModule(PRECOMPILE_P256_VERIFY_EFFECTIVE_CALLS);

  //  related to Modexp
  private final IncrementAndDetectModule modexpEffectiveCall =
      new IncrementAndDetectModule(PRECOMPILE_MODEXP_EFFECTIVE_CALLS);
  private final IncrementingModule modexpLargeCall =
      new IncrementingModule(PRECOMPILE_LARGE_MODEXP_EFFECTIVE_CALLS);

  // related to Rip
  private final RipemdBlocks ripemdBlocks = new RipemdBlocks();

  // related to Blake
  private final IncrementingModule blakeEffectiveCall =
      new IncrementingModule(PRECOMPILE_BLAKE_EFFECTIVE_CALLS);
  private final BlakeRounds blakeRounds = new BlakeRounds();

  // Related to Bls
  final IncrementingModule pointEvaluationEffectiveCall =
      new IncrementingModule(PRECOMPILE_BLS_POINT_EVALUATION_EFFECTIVE_CALLS);
  final IncrementingModule pointEvaluationFailureCall =
      new IncrementingModule(PRECOMPILE_POINT_EVALUATION_FAILURE_EFFECTIVE_CALLS);
  final IncrementingModule blsG1AddEffectiveCall =
      new IncrementingModule(PRECOMPILE_BLS_G1_ADD_EFFECTIVE_CALLS);
  final IncrementingModule blsG1MsmEffectiveCall =
      new IncrementingModule(PRECOMPILE_BLS_G1_MSM_EFFECTIVE_CALLS);
  final IncrementingModule blsG2AddEffectiveCall =
      new IncrementingModule(PRECOMPILE_BLS_G2_ADD_EFFECTIVE_CALLS);
  final IncrementingModule blsG2MsmEffectiveCall =
      new IncrementingModule(PRECOMPILE_BLS_G2_MSM_EFFECTIVE_CALLS);
  final CountingOnlyModule blsPairingCheckMillerLoops =
      new CountingOnlyModule(PRECOMPILE_BLS_PAIRING_CHECK_MILLER_LOOPS);
  final IncrementingModule blsPairingCheckFinalExponentiations =
      new IncrementingModule(PRECOMPILE_BLS_FINAL_EXPONENTIATIONS);
  final IncrementingModule blsG1MapFpToG1EffectiveCall =
      new IncrementingModule(PRECOMPILE_BLS_MAP_FP_TO_G1_EFFECTIVE_CALLS);
  final IncrementingModule blsG1MapFp2ToG2EffectiveCall =
      new IncrementingModule(PRECOMPILE_BLS_MAP_FP2_TO_G2_EFFECTIVE_CALLS);
  final IncrementingModule blsC1MembershipCalls =
      new IncrementingModule(PRECOMPILE_BLS_C1_MEMBERSHIP_CALLS);
  final IncrementingModule blsC2MembershipCalls =
      new IncrementingModule(PRECOMPILE_BLS_C2_MEMBERSHIP_CALLS);
  final IncrementingModule blsG1MembershipCalls =
      new IncrementingModule(PRECOMPILE_BLS_G1_MEMBERSHIP_CALLS);
  final IncrementingModule blsG2MembershipCalls =
      new IncrementingModule(PRECOMPILE_BLS_G2_MEMBERSHIP_CALLS);

  /** Those modules are used only by the sequencer, they don't have associated trace */
  public List<Module> getTracelessModules() {
    return List.of(
        blockTransactions,
        keccak,
        sha256Blocks,
        ecAddEffectiveCall,
        ecMulEffectiveCall,
        ecRecoverEffectiveCall,
        ecPairingG2MembershipCalls,
        ecPairingMillerLoops,
        ecPairingFinalExponentiations,
        modexpEffectiveCall,
        modexpLargeCall,
        ripemdBlocks,
        blakeEffectiveCall,
        blakeRounds,
        pointEvaluationEffectiveCall,
        pointEvaluationFailureCall,
        blsG1AddEffectiveCall,
        blsG1MsmEffectiveCall,
        blsG2AddEffectiveCall,
        blsG2MsmEffectiveCall,
        blsPairingCheckMillerLoops,
        blsPairingCheckFinalExponentiations,
        blsG1MapFpToG1EffectiveCall,
        blsG1MapFp2ToG2EffectiveCall,
        blsC1MembershipCalls,
        blsC2MembershipCalls,
        blsG1MembershipCalls,
        blsG2MembershipCalls,
        p256VerifyEffectiveCalls,
        l1BlockSize,
        l2L1Logs);
  }

  /*
   * precompile-data modules
   * those module are traced (and could be count)
   */
  private final ShakiraData shakiraData;

  private final Blake2f blake2f = new Blake2f();

  @Getter
  private final BlakeModexpData blakeModexpData =
      new BlakeModexpData(
          wcp, modexpEffectiveCall, modexpLargeCall, blakeEffectiveCall, blakeRounds);

  public final EcData ecData =
      new EcData(
          wcp,
          ext,
          ecAddEffectiveCall,
          ecMulEffectiveCall,
          ecRecoverEffectiveCall,
          ecPairingG2MembershipCalls,
          ecPairingMillerLoops,
          ecPairingFinalExponentiations,
          p256VerifyEffectiveCalls);
  final Module blsData =
      new BlsData(
          wcp,
          pointEvaluationEffectiveCall,
          pointEvaluationFailureCall,
          blsG1AddEffectiveCall,
          blsG1MsmEffectiveCall,
          blsG2AddEffectiveCall,
          blsG2MsmEffectiveCall,
          blsPairingCheckMillerLoops,
          blsPairingCheckFinalExponentiations,
          blsG1MapFpToG1EffectiveCall,
          blsG1MapFp2ToG2EffectiveCall,
          blsC1MembershipCalls,
          blsC2MembershipCalls,
          blsG1MembershipCalls,
          blsG2MembershipCalls);

  private final L1BlockSize l1BlockSize;
  private final IncrementingModule l2L1Logs;

  /** list of module than can be modified during execution */
  private final List<Module> modules;

  /** reference table modules */
  private final List<Module> refTableModules;

  /**
   * The real modules, ie the ones that are traced and triggered during execution. It differs with
   * the moduleToTrace() as it contains module traced for some fork only.
   */
  public List<Module> realModule() {
    return List.of(
        this,
        add,
        bin,
        blake2f,
        blakeModexpData,
        blockdata,
        blockhash,
        blsData,
        ecData,
        exp,
        ext,
        euc,
        gas,
        logData,
        logInfo,
        mmu, // WARN: must be traced before the MMIO
        mmio,
        mod,
        mul,
        mxp,
        oob,
        rlpAddr,
        rlpTxn,
        rlpTxnRcpt,
        rlpUtils,
        rom,
        romLex,
        shakiraData,
        shf,
        stp,
        trm,
        txnData,
        wcp);
  }

  /**
   * @return a list of all modules for which to generate traces
   */
  public List<Module> getModulesToTrace() {
    final List<Module> allModules =
        new ArrayList<>(Stream.concat(realModule().stream(), refTableModules.stream()).toList());

    // The coordinator requires to have the same set of module in counting whatever the fork. But we
    // don't trace them.
    return allModules.stream().filter(module -> !(module instanceof CountingOnlyModule)).toList();
  }

  /**
   * List all the modules for which to generate counters. This includes all the tracing modules (
   * {@code getModulesToTrace}) as well as all the so-called traceless modules.
   *
   * @return the modules to count
   */
  public List<Module> getModulesToCount() {
    return Stream.concat(realModule().stream(), getTracelessModules().stream()).toList();
  }

  public Hub(final ChainConfig chain, PublicInputs publicInputs) {
    fork = chain.fork;
    gasCalculator = getGasCalculatorFromFork(fork);
    opCodes = OpCodes.load(fork);
    gasProjector = new GasProjector(fork, gasCalculator);
    checkState(chain.id.signum() >= 0, "Hub constructor: chain id must be non-negative");
    final Address l2l1ContractAddress = chain.bridgeConfiguration.contract();
    final Bytes32 l2l1Topic = chain.bridgeConfiguration.topic();
    if (l2l1ContractAddress.equals(TEST_DEFAULT.contract())) {
      log.info("[ZkTracer] Using default testing L2L1 contract address");
    }
    l2L1Logs = new IncrementingModule(BLOCK_L2_L1_LOGS);
    keccak = new Keccak(ecRecoverEffectiveCall, blockTransactions);
    l1BlockSize =
        new L1BlockSize(
            blockTransactions, keccak, l2L1Logs, l2l1ContractAddress, LogTopic.of(l2l1Topic));
    shakiraData = new ShakiraData(wcp, sha256Blocks, keccak, ripemdBlocks);
    trm = new Trm(fork);
    rlpTxn = new RlpTxn(rlpUtils, trm);
    rlpAddr = new RlpAddr(this, trm, keccak);
    blockdata = new CancunBlockData(this, wcp, euc, chain, publicInputs.blobBaseFees());
    mmu = new Mmu(euc, wcp);
    mmio = new Mmio(mmu);
    blockhash = new Blockhash(this, wcp, publicInputs.historicalBlockhashes());

    refTableModules =
        Stream.of(new BlsRt(), new InstructionDecoder(this.opCodes()))
            .filter(Objects::nonNull)
            .toList();

    modules =
        Stream.concat(
                Stream.of(
                    add,
                    bin,
                    blake2f,
                    blakeModexpData,
                    blockhash, /* WARN: must be called BEFORE WCP (for traceEndConflation) */
                    blsData,
                    ecData,
                    euc,
                    ext,
                    gas,
                    mmio,
                    mmu,
                    mod,
                    mul,
                    mxp,
                    oob,
                    exp,
                    rlpAddr,
                    rlpTxn,
                    rlpTxnRcpt,
                    rlpUtils,
                    logData, /* WARN: must be called AFTER rlpTxnRcpt */
                    logInfo, /* WARN: must be called AFTER rlpTxnRcpt */
                    rom,
                    romLex,
                    shakiraData,
                    shf,
                    stp,
                    trm,
                    wcp, /* WARN: must be called BEFORE txnData */
                    txnData,
                    blockdata /* WARN: must be called AFTER txnData */),
                getTracelessModules().stream())
            .toList();
  }

  @Override
  public void commitTransactionBundle() {
    txStack.commitTransactionBundle();
    state.commitTransactionBundle();

    transients.conflation().stackHeightChecksForStackUnderflows().commitTransactionBundle();
    transients.conflation().stackHeightChecksForStackOverflows().commitTransactionBundle();
    for (Module m : modules) {
      m.commitTransactionBundle();
    }
  }

  @Override
  public void popTransactionBundle() {
    txStack.popTransactionBundle();
    state.popTransactionBundle();
    transients.conflation().stackHeightChecksForStackUnderflows().popTransactionBundle();
    transients.conflation().stackHeightChecksForStackOverflows().popTransactionBundle();
    defers.clearAll();
    for (Module m : modules) {
      m.popTransactionBundle();
    }
  }

  /** Tracing Operation, triggered by Besu hook */
  @Override
  public void traceStartConflation(long blockCount) {
    for (Module m : modules) {
      m.traceStartConflation(blockCount);
    }
  }

  @Override
  public void traceEndConflation(final WorldView world) {
    romLex.determineCodeFragmentIndex();
    txStack.setCodeFragmentIndex(this);
    defers.resolvePostConflation(this, world);

    for (Module m : modules) {
      m.traceEndConflation(world);
    }
  }

  @Override
  public void traceStartBlock(
      WorldView world,
      final ProcessableBlockHeader processableBlockHeader,
      final Address miningBeneficiary) {
    state.firstAndLastStorageSlotOccurrences.add(new HashMap<>());
    blockStack.newBlock(processableBlockHeader, miningBeneficiary);
    txStack.resetBlock();
    state.enterSectionsStack();
    // Compute the line counting of the HUB of the current transaction TODO: this is ugly but will
    // disappear with limitless refacto
    for (Module m : modules) {
      m.traceStartBlock(world, processableBlockHeader, miningBeneficiary);
    }
    traceSysiTransactions(world, processableBlockHeader);
    state.lineCounter().add(state.currentTransactionHubSections().lineCount());

    commitTransactionBundle();
  }

  @Override
  public void traceEndBlock(final BlockHeader blockHeader, final BlockBody blockBody) {
    state.enterSectionsStack();
    traceSystemFinalTransaction();
    // Compute the line counting of the HUB of the current transaction TODO: this is ugly but will
    // disappear with limitless refacto
    state.lineCounter().add(state.currentTransactionHubSections().lineCount());
    for (Module m : modules) {
      m.traceEndBlock(blockHeader, blockBody);
    }
    defers.resolvePostBlock(this);
  }

  public void traceStartTransaction(final WorldView world, final Transaction tx) {
    state.transactionProcessingType(USER);
    pch.reset();
    txStack.enterTransaction(this, world, tx);
    final TransactionProcessingMetadata transactionProcessingMetadata = txStack.current();
    state.enterTransaction();

    if (!transactionProcessingMetadata.requiresEvmExecution()) {
      state.processingPhase(TX_SKIP);
      new TxSkipSection(this, world, transactionProcessingMetadata, transients);
    } else {
      if (transactionProcessingMetadata.requiresPrewarming()) {
        state.processingPhase(TX_WARM);
        new TxPreWarmingMacroSection(world, this);
      }
      state.processingPhase(TX_INIT);
      new TxInitializationSection(this, world);
    }

    // Note: for deployment transactions the deployment number / status were updated during the
    // initialization phase. We are thus capturing the respective XXX_NEW's
    transactionProcessingMetadata
        .captureUpdatedInitialRecipientAddressDeploymentInfoAtTransactionStart(this);

    for (Module m : modules) {
      m.traceStartTx(world, transactionProcessingMetadata);
    }
  }

  // the sender already received its gas refund
  // the coinbase already received its gas reward
  public void traceEndTransaction(
      WorldView world,
      Transaction tx,
      boolean isSuccessful,
      List<Log> logs,
      Set<Address> selfDestructs) {

    txStack.current().completeLineaTransaction(this, world, isSuccessful, logs, selfDestructs);
    defers.resolveAtEndTransaction(this, world, tx, isSuccessful);
    defers.resolveAfterTransactionFinalization(this, world);

    // Warn: we need to call MMIO after resolving the defers
    for (Module m : modules) {
      m.traceEndTx(txStack.current());
    }

    // Compute the line counting of the HUB of the current transaction and add the exceptional
    // fragment
    state.lineCounter().add(state.currentTransactionHubSections().lineCount());
  }

  @Override
  public void traceContextEnter(MessageFrame frame) {
    pch.reset();

    // root and transaction call data context's
    if (frame.getDepth() == 0) {
      if (state.processingPhase() == TX_SKIP) {
        checkState(
            currentTraceSection() instanceof TxSkipSection,
            "traceContextEnter of Hub: expected a skip section");
        ((TxSkipSection) currentTraceSection()).coinbaseSnapshots(this, frame);
      }
      final TransactionProcessingMetadata currentTransaction = transients().tx();
      final Address recipientAddress = frame.getRecipientAddress();
      final Address senderAddress = frame.getSenderAddress();
      final boolean isDeployment = frame.getType() == CONTRACT_CREATION;
      final Wei value = frame.getValue();
      final long initiallyAvailableGas = frame.getRemainingGas();
      final Transaction tx = currentTransaction.getBesuTransaction();

      checkArgument(
          recipientAddress.equals(effectiveToAddress(tx)),
          "Mismatch between frame and transaction recipient");
      checkArgument(
          senderAddress.equals(tx.getSender()), "Mismatch between frame and transaction sender");
      checkArgument(
          isDeployment == tx.getTo().isEmpty(),
          "Mismatch between frame and transaction deployment info");
      checkArgument(
          value.equals(Wei.of(tx.getValue().getAsBigInteger())),
          "Mismatch between frame and transaction value");
      checkArgument(
          frame.getRemainingGas() == currentTransaction.getInitiallyAvailableGas(),
          "Frame gas available at the beginning of the tx %s != transaction initially available gas %s",
          frame.getRemainingGas(),
          currentTransaction.getInitiallyAvailableGas());

      final boolean copyTransactionCallData = currentTransaction.copyTransactionCallData();
      if (copyTransactionCallData) {
        callStack.transactionCallDataContext(
            callDataContextNumber(true), currentTransaction.getBesuTransaction().getData().get());
      }

      callStack.newRootContext(
          newChildContextNumber(),
          senderAddress,
          recipientAddress,
          new Bytecode(
              currentTransaction.isDeployment()
                  ? currentTransaction.getBesuTransaction().getInit().orElse(Bytes.EMPTY)
                  : Optional.ofNullable(frame.getWorldUpdater().get(recipientAddress))
                      .map(AccountState::getCode)
                      .orElse(Bytes.EMPTY)),
          value,
          initiallyAvailableGas,
          callDataContextNumber(copyTransactionCallData),
          transients.tx().getBesuTransaction().getData().orElse(Bytes.EMPTY),
          this.deploymentNumberOf(recipientAddress),
          this.deploymentNumberOf(recipientAddress),
          this.deploymentStatusOf(recipientAddress));

      this.currentFrame().initializeFrame(frame);
    }

    // internal transaction (CALL) or internal deployment (CREATE)
    if (frame.getDepth() > 0) {
      final OpCodeData currentOpCode = opCodes.of(callStack.currentCallFrame().opCode());
      final boolean isDeployment = frame.getType() == CONTRACT_CREATION;

      checkState(
          currentOpCode.isCall() || currentOpCode.isCreate(),
          "trace context enter at positive depth must be call or create");
      checkState(
          currentTraceSection() instanceof CallSection
              || currentTraceSection() instanceof CreateSection,
          "trace context enter at positive depth must have in call or create section as most recent trace section");
      checkState(
          currentTraceSection() instanceof CreateSection == isDeployment,
          "trace context enter at positive depth must have a create section as most recent trace section iff it is a deployment");

      final CallFrameType frameType =
          frame.isStatic() ? CallFrameType.STATIC : CallFrameType.STANDARD;

      final MemoryRange callDataRange =
          isDeployment
              ? new MemoryRange(currentFrame().contextNumber())
              : ((CallSection) currentTraceSection()).getCallDataRange();

      currentFrame().rememberGasNextBeforePausing(this);
      currentFrame().pauseCurrentFrame();

      MemoryRange returnAtRange =
          isDeployment
              ? new MemoryRange(currentFrame().contextNumber())
              : ((CallSection) currentTraceSection()).getReturnAtRange();

      callStack.enter(
          frameType,
          newChildContextNumber(),
          this.deploymentStatusOf(frame.getContractAddress()),
          frame.getApparentValue(),
          frame.getRemainingGas(),
          frame.getRecipientAddress(),
          this.deploymentNumberOf(frame.getRecipientAddress()),
          frame.getContractAddress(),
          this.deploymentNumberOf(frame.getContractAddress()),
          new Bytecode(frame.getCode().getBytes()),
          frame.getSenderAddress(),
          callDataRange,
          returnAtRange);

      this.currentFrame().initializeFrame(frame);

      for (Module m : modules) {
        m.traceContextEnter(frame);
      }
    }

    defers.resolveUponContextEntry(this, frame);
  }

  @Override
  public void traceContextExit(MessageFrame frame) {
    exitDeploymentFromDeploymentInfoPov(frame);

    // We take a snapshot before exiting the transaction
    if (frame.getDepth() == 0) {
      final long leftOverGas = frame.getRemainingGas();
      final long gasRefund = frame.getGasRefund();

      txStack
          .current()
          .setPreFinalisationValues(
              leftOverGas, gasRefund, txStack.getAccumulativeGasUsedInBlockBeforeTxStart());

      if (state.processingPhase() != TX_SKIP) {
        state.processingPhase(TX_FINL);
        new TxFinalizationSection(this);
      }
    }

    defers.resolveUponContextExit(this, this.currentFrame());
    if (this.opCode() == REVERT || Exceptions.any(pch.exceptions())) {
      defers.resolveUponRollback(this, frame, this.currentFrame());
    }

    if (frame.getDepth() > 0) {
      callStack.exit();
    }
  }

  public void traceContextReEnter(MessageFrame frame) {
    // Note: the update of the currentId call frame is made during traceContextExit of the child
    // frame
    defers.resolveUponContextReEntry(this, this.currentFrame());
    this.unlatchStack(frame, this.currentFrame().childSpanningSection());
  }

  public void tracePreExecution(final MessageFrame frame) {
    checkArgument(
        state().processingPhase() == TX_EXEC,
        "There can't be any execution if the HUB is not in execution phase");
    this.processStateExec(frame);
  }

  /**
   * A comment on {@link #unlatchStack(MessageFrame, TraceSection)}: Any instruction that writes
   * onto the stack gets immediately unlatched if it raises an exception. If unexceptional it also
   * gets immediately unlatched, except CALL's and CREATE's. The value written on the stack
   * (<b>successBit</b> or <b>successBit ∙ [child address]</b> respectively) is only written after
   * the child context has been executed.
   *
   * <p><b>Question:</b> Does this work well with CALL's to EOA's ? to PRC's ? trivial deployments
   * (i.e. empty initialization code) ?
   */
  public void tracePostExecution(MessageFrame frame, Operation.OperationResult operationResult) {
    checkArgument(
        state().processingPhase() == TX_EXEC,
        "There can't be any execution if the HUB is not in execution phase");

    final TraceSection currentSection = state.currentTransactionHubSections().currentSection();

    /*
     * NOTE: whenever there is an exception, a context row
     * is added at the end of the section; its purpose is
     * to update the caller / creator context with empty
     * return data.
     */
    if (isExceptional()) {
      this.currentTraceSection()
          .exceptionalContextFragment(ContextFragment.executionProvidesEmptyReturnData(this));
    }

    defers.resolvePostExecution(this, frame, operationResult);

    if (isExceptional() || !opCodeData().isCallOrCreate()) {
      this.unlatchStack(frame, currentSection);
    }
  }

  public boolean isUnexceptional() {
    return currentTraceSection().commonValues.tracedException() == NONE;
  }

  public boolean isExceptional() {
    return !isUnexceptional();
  }

  /**
   * If the current execution context is a deployment context the present method "exits" that
   * deployment in the sense that it updates the relevant deployment information.
   */
  private void exitDeploymentFromDeploymentInfoPov(MessageFrame frame) {
    final Address bytecodeAddress = currentFrame().byteCodeAddress();
    checkArgument(
        bytecodeAddress.equals(bytecodeAddress()),
        "bytecode address mismatch between frame / callFrame at exit from deployment");

    /**
     * Explanation: if the current address isn't under deployment there is nothing to do.
     *
     * <p>If the transaction is of TX_SKIP type then it is a deployment it has empty code and is
     * immediately set to the deployed state
     */
    if (state.processingPhase() == TX_SKIP) {
      checkArgument(
          !deploymentStatusOfBytecodeAddress(), "TX_SKIP: deployments must have empty code");
      return;
    }
    /**
     * We can't say anything if the current frame is a message call: we might have attempted a call
     * to an address that is undergoing deployment (or a normal one.)
     */
    if (frame.getType() == MESSAGE_CALL) {
      return;
    }

    // from here on out:
    // - state.processingPhase != TX_SKIP
    // - messageFrame.type == CONTRACT_CREATION

    /**
     * Note: we can't a priori know the deployment status of an address where a CREATE(2) raised the
     * Failure Condition F. We also do not want to modify its deployment status. Deployment might
     * still be underway, e.g.
     *
     * <p>bytecode A executes CREATE2; bytecode B is the init code; bytecode B executes a CALL to
     * address A; bytecode A executes exactly the same CREATE2 raising the Failure Condition F for
     * address B;
     */
    if (this.currentTraceSection() instanceof CreateSection
        && ((CreateSection) this.currentTraceSection()).scenarioFragment.isFailedCreate()) {
      return;
    }
    // from here on out: no failure condition
    // we must still distinguish between 'empty' deployments and 'nonempty' ones

    final boolean emptyDeployment = messageFrame().getCode().getBytes().isEmpty();

    checkArgument(
        deploymentStatusOfBytecodeAddress() == !emptyDeployment,
        "empty deployments are immediately considered as 'deployed'");

    if (emptyDeployment) return;
    // from here on out nonempty deployments

    // we transition 'nonempty deployments' from 'underDeployment' to 'deployed'
    transients.conflation().deploymentInfo().markAsNotUnderDeployment(bytecodeAddress);
  }

  public int getCodeFragmentIndexByMetaData(
      final Address address,
      final int deploymentNumber,
      final boolean deploymentStatus,
      final int delegationNumber) {
    return this.romLex()
        .getCodeFragmentIndexByMetadata(
            ContractMetadata.make(address, deploymentNumber, deploymentStatus, delegationNumber));
  }

  public int callDataContextNumber(final boolean shouldCopyTxCallData) {
    return shouldCopyTxCallData ? this.stamp() : 0;
  }

  public static int newIdentifierFromStamp(int h) {
    return 1 + h;
  }

  public int newChildContextNumber() {
    return newIdentifierFromStamp(this.stamp());
  }

  public CallFrame currentFrame() {
    return callStack().isEmpty() ? CallFrame.EMPTY : callStack.currentCallFrame();
  }

  public final MessageFrame messageFrame() {
    return callStack.currentCallFrame().frame();
  }

  private void handleStack(MessageFrame frame) {
    this.currentFrame()
        .stack()
        .processInstruction(this, frame, MULTIPLIER___STACK_STAMP * (stamp() + 1));
  }

  public int stamp() {
    return state.stamps().hub();
  }

  /**
   * Return information about the opcode being executed in the current call frame.
   *
   * @return
   */
  public OpCodeData opCodeData() {
    return opCodes.of(this.currentFrame().opCode());
  }

  /**
   * Return information about the opcode being executed in a given message frame.
   *
   * @param frame
   * @return
   */
  public OpCodeData opCodeData(MessageFrame frame) {
    return opCodes.of(frame.getCurrentOperation().getOpcode());
  }

  public OpCode opCode() {
    return opCodeData().mnemonic();
  }

  public TraceSection currentTraceSection() {
    return state.currentTransactionHubSections().currentSection();
  }

  public TraceSection lastUserTransactionSection() {
    return lastUserTransactionSection(1);
  }

  public TraceSection lastUserTransactionSection(int n) {
    return state.lastUserTransactionHubSections().previousSection(n);
  }

  public void addTraceSection(TraceSection section) {
    state.currentTransactionHubSections().add(section);
  }

  public void unlatchStack(MessageFrame frame, TraceSection section) {
    if (this.currentFrame().pending() == null) {
      return;
    }

    final StackContext pending = this.currentFrame().pending();
    for (int i = 0; i < pending.lines().size(); i++) {
      final StackLine line = pending.lines().get(i);

      if (line.needsResult()) {
        Bytes result = Bytes.EMPTY;
        // Note: when we call this from contextReenter, pch.exceptions is the one from the last
        // opcode of the caller/creater ?
        if (Exceptions.none(pch.exceptions())) {
          result = frame.getStackItem(0).copy();
        }

        // This works because we are certain that the stack chunks are the first.
        ((StackFragment) section.fragments().get(i))
            .stackOps()
            .get(line.resultColumn() - 1)
            .value(result);
      }
    }
  }

  void processStateExec(MessageFrame frame) {
    pch.setup(frame);

    this.handleStack(frame);

    if (currentFrame().stack().isOk()) {
      // Tracer for the HUB
      this.traceOpcode(frame);
    } else {
      this.squashCurrentFrameOutputData();
      this.squashParentFrameReturnData();
      new EarlyExceptionSection(this);
    }

    if (Exceptions.any(pch().exceptions()) || opCode() == REVERT) {
      currentFrame().setRevertStamps(callStack, stamp());
    }
  }

  public long remainingGas() {
    return this.state().processingPhase() == TX_EXEC
        ? this.currentFrame().frame().getRemainingGas()
        : 0;
  }

  void traceOpcode(MessageFrame frame) {
    final OpCodeData op = opCodeData();
    switch (op.instructionFamily()) {
      case ADD, BIN, MOD, SHF, WCP, EXT, BATCH, PUSH_POP, DUP, SWAP ->
          new StackOnlySection(this, op);
      case MACHINE_STATE -> {
        switch (op.mnemonic()) {
          case MSIZE -> new MsizeSection(this);
          default -> new StackOnlySection(this, op);
        }
      }
      case MUL -> {
        switch (op.mnemonic()) {
          case EXP -> new ExpSection(this);
          case MUL -> new StackOnlySection(this, op);
          default ->
              throw new IllegalStateException(
                  String.format("opcode %s not part of the MUL instruction family", this.opCode()));
        }
      }
      case HALT -> {
        switch (op.mnemonic()) {
          case RETURN -> new ReturnSection(this, frame);
          case REVERT -> new RevertSection(this, frame);
          case STOP -> new StopSection(this);
          case SELFDESTRUCT -> new SelfdestructSection(this, frame);
        }
        final boolean returnFromDeployment =
            (op.mnemonic() == RETURN && this.currentFrame().isDeployment());

        callStack
            .parentCallFrame()
            .returnDataRange(
                returnFromDeployment
                    ? new MemoryRange(currentFrame().contextNumber())
                    : currentFrame().outputDataRange());
      }
      case KEC -> new KeccakSection(this);
      case CONTEXT -> new ContextSection(this);
      case LOG -> new LogSection(this);
      case ACCOUNT -> new AccountSection(this);
      case COPY -> {
        switch (op.mnemonic()) {
          case CALLDATACOPY -> new CallDataCopySection(this);
          case RETURNDATACOPY -> new ReturnDataCopySection(this);
          case CODECOPY -> new CodeCopySection(this);
          case EXTCODECOPY -> new ExtCodeCopySection(this, frame);
          default ->
              throw new RuntimeException(
                  "Invalid instruction: " + this.opCode().toString() + " not in the COPY family");
        }
      }
      case MCOPY -> new McopySection(this);
      case TRANSACTION -> new TransactionSection(this);
      case STACK_RAM -> {
        switch (op.mnemonic()) {
          case CALLDATALOAD -> new CallDataLoadSection(this);
          case MLOAD, MSTORE, MSTORE8 -> new StackRamSection(this);
          default -> throw new IllegalStateException("unexpected STACK_RAM opcode");
        }
      }
      case STORAGE -> {
        switch (op.mnemonic()) {
          case SSTORE -> new SstoreSection(this, frame.getWorldUpdater());
          case SLOAD -> new SloadSection(this, frame.getWorldUpdater());
          default -> throw new IllegalStateException("invalid operation in family STORAGE");
        }
      }
      case TRANSIENT -> {
        switch (op.mnemonic()) {
          case TLOAD -> new TLoadSection(this);
          case TSTORE -> new TStoreSection(this);
          default ->
              throw new IllegalStateException(
                  "invalid operation in family TRANSIENT: " + op.mnemonic());
        }
      }
      case JUMP -> new JumpSection(this);
      case CREATE -> new CreateSection(this, frame);
      case CALL -> new CallSection(this, frame);
      case INVALID -> new EarlyExceptionSection(this);
    }
  }

  public void squashCurrentFrameOutputData() {
    callStack.currentCallFrame().outputDataRange(MemoryRange.EMPTY);
  }

  public void squashParentFrameReturnData() {
    callStack.parentCallFrame().returnDataRange(MemoryRange.EMPTY);
  }

  public CallFrame getLastChildCallFrame(final CallFrame parentFrame) {
    return callStack.getById(parentFrame.childFrameIds().getLast());
  }

  // Quality of life deployment info related functions
  public int deploymentNumberOf(Address address) {
    return transients.conflation().deploymentInfo().deploymentNumber(address);
  }

  public boolean deploymentStatusOf(Address address) {
    return transients.conflation().deploymentInfo().getDeploymentStatus(address);
  }

  public int delegationNumberOf(Address address) {
    return transients.conflation().getDelegationNumber(address);
  }

  // methods related to the byte code address
  // (c in the definition of \Theta in the EYP)
  public Address bytecodeAddress() {
    return this.messageFrame().getContractAddress();
  }

  public int deploymentNumberOfBytecodeAddress() {
    return deploymentNumberOf(bytecodeAddress());
  }

  public boolean deploymentStatusOfBytecodeAddress() {
    return deploymentStatusOf(bytecodeAddress());
  }

  // methods related to the account address
  // (r in the definition of \Theta in the EYP)
  // (also I_a in the EYP)
  public Address accountAddress() {
    return this.messageFrame().getRecipientAddress();
  }

  public int deploymentNumberOfAccountAddress() {
    return deploymentNumberOf(this.accountAddress());
  }

  public Address coinbaseAddress() {
    return blockStack.currentBlock().coinbaseAddress();
  }

  public Address coinbaseAddressOfRelativeBlock(final int relativeBlockNumber) {
    return blockStack.getBlockByRelativeBlockNumber(relativeBlockNumber).coinbaseAddress();
  }

  private void traceSysiTransactions(WorldView world, ProcessableBlockHeader blockHeader) {
    state.transactionProcessingType(SYSI);
    state.processingPhase(TX_SKIP);

    // Cancun SYSI: EIP4788: Beacon Block Root
    state.incrementSysiTransactionNumber();
    new EIP4788BeaconBlockRootSection(this, world, blockHeader);
    // Prague SYSI: EIP2935: Block Hash
    state.incrementSysiTransactionNumber();
    new EIP2935HistoricalHash(this, world, blockHeader);
  }

  private void traceSystemFinalTransaction() {
    state.transactionProcessingType(SYSF);
    state.incrementSysfTransactionNumber();
    state.processingPhase(TX_SKIP);
    new SysfNoopSection(this);
  }
}
