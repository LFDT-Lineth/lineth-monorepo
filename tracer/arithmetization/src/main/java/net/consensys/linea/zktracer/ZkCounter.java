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

import static net.consensys.linea.zktracer.Fork.*;
import static net.consensys.linea.zktracer.Trace.BLOCKHASH_MAX_HISTORY;
import static net.consensys.linea.zktracer.Trace.Ecdata.TOTAL_SIZE_ECPAIRING_DATA_MIN;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY;
import static net.consensys.linea.zktracer.module.ModuleName.*;
import static net.consensys.linea.zktracer.module.ModuleName.ADD;
import static net.consensys.linea.zktracer.module.ModuleName.EXP;
import static net.consensys.linea.zktracer.module.ModuleName.GAS;
import static net.consensys.linea.zktracer.module.ModuleName.MOD;
import static net.consensys.linea.zktracer.module.ModuleName.MUL;
import static net.consensys.linea.zktracer.module.add.AddOperation.NB_ROWS_ADD;
import static net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeModexpDataOperation.*;
import static net.consensys.linea.zktracer.module.blockdata.module.CancunBlockData.NB_ROWS_BLOCK_DATA;
import static net.consensys.linea.zktracer.module.blockhash.BlockhashOperation.NB_ROWS_BLOCKHASH;
import static net.consensys.linea.zktracer.module.gas.GasOperation.NB_ROWS_GAS;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.*;
import static net.consensys.linea.zktracer.module.hub.section.AccountSection.NB_ROWS_HUB_ACCOUNT;
import static net.consensys.linea.zktracer.module.hub.section.CallDataLoadSection.NB_ROWS_HUB_CALLDATALOAD;
import static net.consensys.linea.zktracer.module.hub.section.CreateSection.NB_ROWS_HUB_CREATE;
import static net.consensys.linea.zktracer.module.hub.section.JumpSection.NB_ROWS_HUB_JUMP;
import static net.consensys.linea.zktracer.module.hub.section.McopySection.NB_ROWS_HUB_MCOPY;
import static net.consensys.linea.zktracer.module.hub.section.MsizeSection.NB_ROWS_HUB_MSIZE;
import static net.consensys.linea.zktracer.module.hub.section.SstoreSection.NB_ROWS_HUB_STORAGE;
import static net.consensys.linea.zktracer.module.hub.section.StackOnlySection.NB_ROWS_HUB_SIMPLE_STACK_OP;
import static net.consensys.linea.zktracer.module.hub.section.StackRamSection.NB_ROWS_HUB_STACKRAM;
import static net.consensys.linea.zktracer.module.hub.section.TxFinalizationSection.NB_ROWS_HUB_FINL;
import static net.consensys.linea.zktracer.module.hub.section.TxInitializationSection.NB_ROWS_HUB_INIT;
import static net.consensys.linea.zktracer.module.hub.section.TxSkipSection.NB_ROWS_HUB_SKIP;
import static net.consensys.linea.zktracer.module.hub.section.call.CallSection.NB_ROWS_HUB_CALL;
import static net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.BlakeSubsection.NB_ROWS_HUB_PRC_BLAKE;
import static net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.EllipticCurvePrecompileSubsection.NB_ROWS_HUB_PRC_ELLIPTIC_CURVE;
import static net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.IdentitySubsection.NB_ROWS_HUB_PRC_IDENTITY;
import static net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.ModexpSubsection.NB_ROWS_HUB_PRC_MODEXP;
import static net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.ShaTwoOrRipemdSubSection.NB_ROWS_HUB_PRC_SHARIP;
import static net.consensys.linea.zktracer.module.hub.section.copy.CallDataCopySection.NB_ROWS_HUB_CALL_DATA_COPY;
import static net.consensys.linea.zktracer.module.hub.section.copy.CodeCopySection.NB_ROWS_HUB_CODE_COPY;
import static net.consensys.linea.zktracer.module.hub.section.copy.ExtCodeCopySection.NB_ROWS_HUB_EXT_CODE_COPY;
import static net.consensys.linea.zktracer.module.hub.section.copy.ReturnDataCopySection.NB_ROWS_HUB_RETURN_DATA_COPY;
import static net.consensys.linea.zktracer.module.hub.section.halt.RevertSection.NB_ROWS_HUB_REVERT;
import static net.consensys.linea.zktracer.module.hub.section.halt.SelfdestructSection.NB_ROWS_HUB_SELFDESTRUCT;
import static net.consensys.linea.zktracer.module.hub.section.halt.StopSection.NB_ROWS_HUB_STOP_DEPLOYMENT;
import static net.consensys.linea.zktracer.module.hub.section.halt.StopSection.NB_ROWS_HUB_STOP_MSG_CALL;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP2935HistoricalHash.NB_ROWS_HUB_SYSI_EIP2935;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP4788BeaconBlockRootSection.NB_ROWS_HUB_SYSI_EIP4788;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.SysfNoopSection.NB_ROWS_HUB_SYSF_NOOP;
import static net.consensys.linea.zktracer.module.hub.section.transients.TLoadSection.NB_ROWS_HUB_TLOAD;
import static net.consensys.linea.zktracer.module.hub.section.transients.TStoreSection.NB_ROWS_HUB_TSTORE;
import static net.consensys.linea.zktracer.module.logdata.LogData.lineCountForLogData;
import static net.consensys.linea.zktracer.module.loginfo.LogInfo.lineCountForLogInfo;
import static net.consensys.linea.zktracer.module.mxp.moduleCall.CancunMSizeMxpCall.NB_ROWS_MXP_MSIZE;
import static net.consensys.linea.zktracer.module.mxp.moduleCall.CancunStateUpdateMxpCall.NB_ROWS_MXP_UPDT_B;
import static net.consensys.linea.zktracer.module.mxp.moduleCall.CancunStateUpdateWordPricingMxpCall.NB_ROWS_MXP_UPDT_W;
import static net.consensys.linea.zktracer.module.rlpaddr.RlpAddrOperation.*;
import static net.consensys.linea.zktracer.module.rlptxrcpt.RlpTxrcptOperation.lineCountForRlpTxnRcpt;
import static net.consensys.linea.zktracer.module.shakiradata.ShakiraDataOperation.NB_ROWS_SHAKIRA_RESULT;
import static net.consensys.linea.zktracer.module.stp.StpOperation.NB_ROWS_STP;
import static net.consensys.linea.zktracer.module.txndata.transactions.SysfNoopTransaction.NB_ROWS_TXN_DATA_SYSF_NOOP;
import static net.consensys.linea.zktracer.module.txndata.transactions.SysiEip2935Transaction.NB_ROWS_TXN_DATA_SYSI_EIP2935;
import static net.consensys.linea.zktracer.module.txndata.transactions.SysiEip4788Transaction.NB_ROWS_TXN_DATA_SYSI_EIP4788;
import static net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction.NB_ROWS_TXN_DATA_OSAKA_USER_1559_SEMANTIC;
import static net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction.NB_ROWS_TXN_DATA_OSAKA_USER_NO_1559_SEMANTIC;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.runtime.stack.Stack.MAX_STACK_SIZE;
import static net.consensys.linea.zktracer.types.TransactionProcessingMetadata.computeRequiresEvmExecution;
import static net.consensys.linea.zktracer.types.TransactionUtils.transactionHasEip1559GasSemantics;
import static net.consensys.linea.zktracer.types.Utils.fromDataSizeToLimbNbRows;
import static org.hyperledger.besu.evm.frame.MessageFrame.State.COMPLETED_SUCCESS;

import java.util.*;
import java.util.stream.Stream;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.zktracer.container.module.CountingOnlyModule;
import net.consensys.linea.zktracer.container.module.IncrementAndDetectModule;
import net.consensys.linea.zktracer.container.module.IncrementingModule;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.module.blsdata.BlsData;
import net.consensys.linea.zktracer.module.ecdata.EcData;
import net.consensys.linea.zktracer.module.ext.Ext;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.module.limits.BlockTransactions;
import net.consensys.linea.zktracer.module.limits.Keccak;
import net.consensys.linea.zktracer.module.limits.L1BlockSize;
import net.consensys.linea.zktracer.module.limits.precompiles.BlakeRounds;
import net.consensys.linea.zktracer.module.limits.precompiles.Sha256Blocks;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.OpCodes;
import net.consensys.linea.zktracer.types.MemoryRange;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.ExceptionalHaltReason;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.internal.Words;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.log.LogTopic;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;

@Slf4j
public class ZkCounter implements LineCountingTracer {

  public final Fork fork;
  private final boolean countHistoricalBlockHashes;

  private final OpCodes opCodes;
  private final Trace trace;

  // traced modules
  final CountingOnlyModule add = new CountingOnlyModule(ADD);
  final CountingOnlyModule blakemodexp;
  final CountingOnlyModule blockData;
  final CountingOnlyModule blockHash;
  final BlsData blsdata;
  final EcData ecdata;
  final CountingOnlyModule euc = new CountingOnlyModule(EUC);
  final CountingOnlyModule exp = new CountingOnlyModule(EXP);
  final Ext ext = new Ext();
  final CountingOnlyModule gas;
  final CountingOnlyModule hub;
  final CountingOnlyModule logData;
  final CountingOnlyModule logInfo;
  final CountingOnlyModule mmio;
  final CountingOnlyModule mmu;
  final CountingOnlyModule mod = new CountingOnlyModule(MOD);
  final CountingOnlyModule mul = new CountingOnlyModule(MUL);
  final CountingOnlyModule mxp;
  final CountingOnlyModule oob = new CountingOnlyModule(OOB);
  final CountingOnlyModule rlpAddr;
  final CountingOnlyModule rlpTxn;
  final CountingOnlyModule rlpTxnRcpt;
  final CountingOnlyModule rlpUtils;
  final CountingOnlyModule rom;
  final CountingOnlyModule romlex;
  final CountingOnlyModule shakiradata;
  final CountingOnlyModule shf = new CountingOnlyModule(SHF);
  final IncrementingModule stp = new IncrementingModule(STP);
  final CountingOnlyModule trm;
  final CountingOnlyModule txnData;
  final Wcp wcp = new Wcp();

  // precompiles limits:
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

  // related to Blake
  private final IncrementingModule blakeEffectiveCall =
      new IncrementingModule(PRECOMPILE_BLAKE_EFFECTIVE_CALLS);
  private final BlakeRounds blakeRounds = new BlakeRounds();

  // related to Shakira:
  private final Keccak keccak;
  private final Sha256Blocks sha256Blocks = new Sha256Blocks();
  private final CountingOnlyModule ripemdBlocks = new CountingOnlyModule(PRECOMPILE_RIPEMD_BLOCKS);

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

  // others:
  private final BlockTransactions blockTransactions = new BlockTransactions();
  final L1BlockSize l1BlockSize;
  final IncrementingModule l2l1Logs = new IncrementingModule(BLOCK_L2_L1_LOGS);

  // all modules
  final List<Module> moduleToCount;

  // The line counting for those modules is known to be incomplete / inaccurate
  public List<Module> uncheckedModules() {
    return List.of(
        add,
        euc, // need MMU
        exp,
        ext,
        mmio, // need MMU
        mmu, // not trivial
        mod,
        mul,
        oob, // need to duplicate to have an accurate counting. We have *10 line count if not.
        rlpTxn, // need a refacto to have rlpTxn using not only TransactionProcessingMetadata
        rlpUtils, // need RLP_TXN
        rom, // not trivial
        romlex,
        shf,
        trm, // not trivial
        wcp, // need MMU/TxnData/Oob etc ... to be counted
        // traceless modules
        blakeRounds // blakeEffectiveCall is counted and already rejects all BLAKE calls
        );
  }

  // The line counting for those modules are supposed to be accurate
  public List<Module> checkedModules() {
    return List.of(
        blakemodexp,
        blockData,
        blockHash,
        blsdata,
        ecdata,
        gas,
        hub,
        logData,
        logInfo,
        mxp,
        rlpAddr,
        rlpTxnRcpt,
        shakiradata,
        txnData,
        stp,
        // traceless modules
        ecAddEffectiveCall,
        ecMulEffectiveCall,
        ecRecoverEffectiveCall,
        ecPairingG2MembershipCalls,
        ecPairingMillerLoops,
        ecPairingFinalExponentiations,
        sha256Blocks,
        ripemdBlocks,
        blockTransactions,
        keccak,
        modexpEffectiveCall,
        modexpLargeCall,
        blakeEffectiveCall,
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
        l2l1Logs);
  }

  public ZkCounter(LineaL1L2BridgeSharedConfiguration bridgeConfiguration, Fork fork) {
    this(bridgeConfiguration, fork, true);
  }

  public ZkCounter(
      LineaL1L2BridgeSharedConfiguration bridgeConfiguration,
      Fork fork,
      boolean countHistoricalBlockHashes) {
    this.fork = fork;
    if (forkPredatesOsaka(fork)) {
      throw new IllegalArgumentException("Fork no more supported by the tracer: " + fork);
    }
    this.countHistoricalBlockHashes = countHistoricalBlockHashes;
    opCodes = OpCodes.load(fork);
    trace = getTraceFromFork(fork);
    blakemodexp = new CountingOnlyModule(BLAKE_MODEXP_DATA, trace.blake2fmodexpdata().spillage());
    blockData = new CountingOnlyModule(BLOCK_DATA, trace.blockdata().spillage());
    blockHash = new CountingOnlyModule(BLOCK_HASH, trace.blockhash().spillage());
    gas = new CountingOnlyModule(GAS, trace.gas().spillage());
    hub = new CountingOnlyModule(HUB, trace.hub().spillage());
    logData = new CountingOnlyModule(LOG_DATA, trace.logdata().spillage());
    logInfo = new CountingOnlyModule(LOG_INFO, trace.loginfo().spillage());
    mmio = new CountingOnlyModule(MMIO, trace.mmio().spillage());
    mmu = new CountingOnlyModule(MMU, trace.mmu().spillage());
    mxp = new CountingOnlyModule(MXP, trace.mxp().spillage());
    rlpAddr = new CountingOnlyModule(RLP_ADDR, trace.rlpaddr().spillage());
    rlpTxn = new CountingOnlyModule(RLP_TXN, trace.rlptxn().spillage());
    rlpTxnRcpt = new CountingOnlyModule(RLP_TXN_RCPT, trace.rlptxrcpt().spillage());
    rlpUtils = new CountingOnlyModule(RLP_UTILS, trace.rlputils().spillage());
    rom = new CountingOnlyModule(ROM, trace.rom().spillage());
    romlex = new CountingOnlyModule(ROM_LEX, trace.romlex().spillage());
    shakiradata = new CountingOnlyModule(SHAKIRA_DATA, trace.shakiradata().spillage());
    trm = new CountingOnlyModule(TRM, trace.trm().spillage());
    txnData = new CountingOnlyModule(TXN_DATA, trace.txndata().spillage());
    keccak = new Keccak(ecRecoverEffectiveCall, blockTransactions);
    ecdata =
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
    blsdata =
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
    l1BlockSize =
        new L1BlockSize(
            blockTransactions,
            keccak,
            l2l1Logs,
            bridgeConfiguration.contract(),
            LogTopic.of(bridgeConfiguration.topic()));
    moduleToCount = Stream.concat(checkedModules().stream(), uncheckedModules().stream()).toList();

    log.info("[ZkCounter] Created ZkCounter for fork {}", fork);
    if (!countHistoricalBlockHashes) {
      log.info(
          "[ZkCounter] Historical BlockHashes not counted for BLOCKHASH, might miss 256*6 lines that are common for the whole conflation.");
    }
  }

  @Override
  public void traceStartConflation(long numBlocksInConflation) {
    if (countHistoricalBlockHashes) {
      blockHash.updateTally(
          (int) ((BLOCKHASH_MAX_HISTORY + numBlocksInConflation) * NB_ROWS_BLOCKHASH));
    }
  }

  @Override
  public void traceEndConflation(WorldView state) {}

  @Override
  public void traceStartBlock(
      final WorldView world,
      final BlockHeader blockHeader,
      final BlockBody blockBody,
      final Address miningBeneficiary) {
    l1BlockSize.traceStartBlock(world, blockHeader, miningBeneficiary);
    blockData.updateTally(NB_ROWS_BLOCK_DATA);
    hub.updateTally(NB_ROWS_HUB_SYSI_EIP4788);
    txnData.updateTally(NB_ROWS_TXN_DATA_SYSI_EIP4788);
    hub.updateTally(NB_ROWS_HUB_SYSI_EIP2935);
    txnData.updateTally(NB_ROWS_TXN_DATA_SYSI_EIP2935);
    hub.updateTally(NB_ROWS_HUB_SYSF_NOOP);
    txnData.updateTally(NB_ROWS_TXN_DATA_SYSF_NOOP);

    commitTransactionBundle();
  }

  @Override
  public void tracePrepareTransaction(WorldView worldView, Transaction tx) {
    switch (tx.getType()) {
      case FRONTIER, ACCESS_LIST, EIP1559, DELEGATE_CODE -> {
        blockTransactions.traceStartTx(null, null);
        final boolean triggersEvm = computeRequiresEvmExecution(worldView, tx);
        if (tx.getType().supportsDelegateCode()) {
          // TODO count the delegation phase size of the HUB
        }
        if (triggersEvm) {
          hub.updateTally(NB_ROWS_HUB_INIT + NB_ROWS_HUB_FINL);
          if (tx.getAccessList().isPresent()) {
            final int nRowsWarmPhase =
                tx.getAccessList().get().stream()
                    .mapToInt(listEntry -> listEntry.storageKeys().size() + 1)
                    .sum();
            hub.updateTally(nRowsWarmPhase);
          }
        } else {
          hub.updateTally(NB_ROWS_HUB_SKIP);
        }
        txnData.updateTally(
            transactionHasEip1559GasSemantics(tx)
                ? NB_ROWS_TXN_DATA_OSAKA_USER_1559_SEMANTIC
                : NB_ROWS_TXN_DATA_OSAKA_USER_NO_1559_SEMANTIC);
        // deploymentTransaction:
        if (tx.isContractCreation()) {
          rlpAddr.updateTally(NB_ROWS_RLPADDR_CREATE);
          keccak.updateTally(MAX_SIZE_RLP_HASH_CREATE);
        }
      }
      case BLOB ->
          throw new IllegalStateException(
              "Arithmetization doesn't support tx type: " + tx.getType());
      default -> throw new IllegalArgumentException("tx type unknown: " + tx.getType());
    }
  }

  @Override
  public void traceEndTransaction(
      WorldView worldView,
      Transaction tx,
      boolean status,
      Bytes output,
      List<Log> logs,
      long gasUsed,
      Set<Address> selfDestructs,
      long timeNs) {
    l1BlockSize.traceEndTx(tx, logs);
    rlpTxnRcpt.updateTally(lineCountForRlpTxnRcpt(logs));
    logData.updateTally(lineCountForLogData(logs));
    logInfo.updateTally(lineCountForLogInfo(logs));
  }

  @Override
  public void tracePreExecution(final MessageFrame frame) {
    final OpCodeData opcode = opCodes.of(frame.getCurrentOperation().getOpcode());

    switch (opcode.instructionFamily()) {
      case PUSH_POP, DUP, SWAP, INVALID, ADD, MOD, SHF, BIN, WCP, EXT ->
          hub.updateTally(NB_ROWS_HUB_SIMPLE_STACK_OP);
      case BATCH -> {
        hub.updateTally(NB_ROWS_HUB_SIMPLE_STACK_OP);
        if (opcode.mnemonic() == BLOCKHASH) {
          blockHash.updateTally(NB_ROWS_BLOCKHASH);
        }
      }
      case MACHINE_STATE -> {
        if (opcode.mnemonic() == MSIZE) {
          hub.updateTally(NB_ROWS_HUB_MSIZE);
          mxp.updateTally(NB_ROWS_MXP_MSIZE);
        } else {
          hub.updateTally(NB_ROWS_HUB_SIMPLE_STACK_OP);
        }
      }
      case MUL -> {
        switch (opcode.mnemonic()) {
          case OpCode.EXP -> hub.updateTally(NB_ROWS_HUB_SIMPLE_STACK_OP + 1);
          case OpCode.MUL -> hub.updateTally(NB_ROWS_HUB_SIMPLE_STACK_OP);
        }
      }
      case HALT -> {
        switch (opcode.mnemonic()) {
          case RETURN -> {
            hub.updateTally(7);
            mxp.updateTally(NB_ROWS_MXP_UPDT_W);
            // oob.updateTally(NB_ROWS_OOB_DEPLOYMENT);
            // MMU
            // Note: the unexceptional RETURN_FROM_DEPLOYMENT case is handled in
            // traceAccountCreationResult()
          }
          case REVERT -> {
            hub.updateTally(NB_ROWS_HUB_REVERT);
            mxp.updateTally(NB_ROWS_MXP_UPDT_W);
            // MMU
          }
          case STOP ->
              hub.updateTally(
                  frame.getType() == MessageFrame.Type.MESSAGE_CALL
                      ? NB_ROWS_HUB_STOP_MSG_CALL
                      : NB_ROWS_HUB_STOP_DEPLOYMENT);
          case SELFDESTRUCT -> hub.updateTally(NB_ROWS_HUB_SELFDESTRUCT);
        }
      }
      case KEC -> {
        hub.updateTally(NB_ROWS_HUB_SIMPLE_STACK_OP + 1);
        mxp.updateTally(NB_ROWS_MXP_UPDT_W);

        final boolean stackException = stackException(frame);
        if (stackException) {
          return;
        }

        final int sizeToHash = Words.clampedToInt(frame.getStackItem(1));
        if (sizeToHash != 0) {
          // MMU
          shakiradata.updateTally(fromDataSizeToLimbNbRows(sizeToHash) + NB_ROWS_SHAKIRA_RESULT);
          keccak.updateTally(sizeToHash);
        }
      }
      case CONTEXT, TRANSACTION -> hub.updateTally(NB_ROWS_HUB_SIMPLE_STACK_OP + 1);
      case LOG -> {
        hub.updateTally(opcode.numberOfStackRows() + 2); // CON + MISC
        mxp.updateTally(NB_ROWS_MXP_UPDT_W);
        // MMU
        // Note: nothing to do for LOG info / data / rlp, done at the end of the tx
      }
      case ACCOUNT -> hub.updateTally(NB_ROWS_HUB_ACCOUNT);
      case COPY -> {
        switch (opcode.mnemonic()) {
          case CALLDATACOPY -> {
            hub.updateTally(NB_ROWS_HUB_CALL_DATA_COPY);
            mxp.updateTally(NB_ROWS_MXP_UPDT_W);
            // MMU
          }
          case RETURNDATACOPY -> {
            hub.updateTally(NB_ROWS_HUB_RETURN_DATA_COPY);
            // oob.updateTally(NB_ROWS_OOB_RDC);
            add.updateTally(NB_ROWS_ADD); // coming from OOB call
            mxp.updateTally(NB_ROWS_MXP_UPDT_W);
            // MMU
          }
          case CODECOPY -> {
            hub.updateTally(NB_ROWS_HUB_CODE_COPY);
            mxp.updateTally(NB_ROWS_MXP_UPDT_W);
            // MMU
            // ROM
          }
          case EXTCODECOPY -> {
            hub.updateTally(NB_ROWS_HUB_EXT_CODE_COPY);
            mxp.updateTally(NB_ROWS_MXP_UPDT_W);
            // MMU
            // ROM
          }
        }
      }
      case MCOPY -> {
        hub.updateTally(NB_ROWS_HUB_MCOPY);
        mxp.updateTally(NB_ROWS_MXP_UPDT_W);
        // MMU
      }
      case STACK_RAM -> {
        switch (opcode.mnemonic()) {
          case CALLDATALOAD -> {
            hub.updateTally(NB_ROWS_HUB_CALLDATALOAD);
            // oob.updateTally(NB_ROWS_OOB_CDL);
            // MMU
          }
          case MSTORE, MLOAD -> {
            hub.updateTally(NB_ROWS_HUB_STACKRAM);
            mxp.updateTally(NB_ROWS_MXP_UPDT_W);
            // MMU
          }
          case MSTORE8 -> {
            hub.updateTally(NB_ROWS_HUB_STACKRAM);
            mxp.updateTally(NB_ROWS_MXP_UPDT_B);
            // MMU
          }
        }
      }
      case STORAGE -> {
        hub.updateTally(NB_ROWS_HUB_STORAGE);
        // if (opcode.mnemonic() == SSTORE) {
        // oob.updateTally(NB_ROWS_OOB_SSTORE);
        // }
      }
      case TRANSIENT -> {
        switch (opcode.mnemonic()) {
          case TLOAD -> hub.updateTally(NB_ROWS_HUB_TLOAD);
          case TSTORE -> hub.updateTally(NB_ROWS_HUB_TSTORE);
        }
      }
      case JUMP -> {
        hub.updateTally(NB_ROWS_HUB_JUMP);
        // oob.updateTally(opcode.mnemonic() == JUMPI ? NB_ROWS_OOB_JUMPI : NB_ROWS_OOB_JUMP);
      }
      case CREATE -> {
        // ROM
        hub.updateTally(NB_ROWS_HUB_CREATE);
        gas.updateTally(NB_ROWS_GAS); // as CMC == 1
        // first IMC
        stp.updateTally(NB_ROWS_STP);
        mxp.updateTally(NB_ROWS_MXP_UPDT_W);
        // oob.updateTally(CT_MAX_CREATE + 1);
        // MMU
        switch (opcode.mnemonic()) {
          case CREATE -> {
            rlpAddr.updateTally(NB_ROWS_RLPADDR_CREATE);
            keccak.updateTally(MAX_SIZE_RLP_HASH_CREATE);
          }
          case CREATE2 -> {
            rlpAddr.updateTally(NB_ROWS_RLPADDR_CREATE2);
            keccak.updateTally(MAX_SIZE_RLP_HASH_CREATE2);
            final boolean stackException = stackException(frame);
            if (stackException) {
              return;
            }

            final int size = Words.clampedToInt(frame.getStackItem(2));
            shakiradata.updateTally(fromDataSizeToLimbNbRows(size) + NB_ROWS_SHAKIRA_RESULT);
            keccak.updateTally(size);
          }
          default -> throw new IllegalArgumentException(opcode + "is not of CREATE family");
        }
      }
      case CALL -> {
        hub.updateTally(NB_ROWS_HUB_CALL);
        gas.updateTally(NB_ROWS_GAS); // as CMC == 1
        // oob.updateTally(CT_MAX_CALL + 1);
        mxp.updateTally(NB_ROWS_MXP_UPDT_W);
        stp.updateTally(NB_ROWS_STP);
        // Note: precompiles specific limits are done in tracePrecompileCall()
      }
      default -> throw new IllegalArgumentException("Unknown opcode: " + opcode.byteValue());
    }
  }

  @Override
  public void traceContextExit(final MessageFrame frame) {
    hub.updateTally(1); // One context row in case of exception
    gas.updateTally(NB_ROWS_GAS); // One gas row in case of exception
  }

  @Override
  public void traceAccountCreationResult(
      final MessageFrame frame, final Optional<ExceptionalHaltReason> haltReason) {
    if (frame.getCurrentOperation() != null
        && frame.getCurrentOperation().getOpcode() == RETURN.getOpcode()) {
      shakiradata.updateTally(
          fromDataSizeToLimbNbRows((int) frame.memoryByteSize()) + NB_ROWS_SHAKIRA_RESULT);
    }
    keccak.updateTally((int) frame.memoryByteSize());
  }

  @Override
  public void tracePrecompileCall(MessageFrame frame, long gasRequirement, Bytes output) {
    final PrecompileScenarioFragment.PrecompileFlag precompile =
        addressToPrecompileFlag(frame.getContractAddress());
    final Bytes callData = frame.getInputData();
    final int callDataSize = callData.size();
    final boolean prcSuccess = frame.getState() == COMPLETED_SUCCESS;
    final Bytes returnData = output == null ? Bytes.EMPTY : output;

    switch (precompile) {
      case PRC_ECRECOVER, PRC_ECADD, PRC_ECMUL -> {
        // trigger EcData to count the underlying EC operations
        if (callDataSize != 0) {
          // Note: we can't know the id (and we don't care)
          ecdata.callEcData(0, precompile, frame.getInputData(), returnData);
        }
        hub.updateTally(NB_ROWS_HUB_PRC_ELLIPTIC_CURVE);
        // oob.updateTally(oobLineCountForPrc(precompile));
      }
      case PRC_SHA2_256 -> {
        hub.updateTally(NB_ROWS_HUB_PRC_SHARIP);
        // oob.updateTally(oobLineCountForPrc(precompile));
        // mod.updateTally(modLinesComingFromOobCall(precompile));
        if (prcSuccess && callDataSize != 0) {
          shakiradata.updateTally(fromDataSizeToLimbNbRows(callDataSize) + NB_ROWS_SHAKIRA_RESULT);
          sha256Blocks.updateTally(callData.size());
        }
      }
      case PRC_RIPEMD_160 -> {
        hub.updateTally(NB_ROWS_HUB_PRC_SHARIP);
        // oob.updateTally(oobLineCountForPrc(precompile));
        // mod.updateTally(modLinesComingFromOobCall(precompile));
        if (prcSuccess && callDataSize != 0) {
          shakiradata.updateTally(fromDataSizeToLimbNbRows(callDataSize) + NB_ROWS_SHAKIRA_RESULT);
          ripemdBlocks.updateTally(callDataSize);
        }
      }
      case PRC_IDENTITY -> {
        hub.updateTally(NB_ROWS_HUB_PRC_IDENTITY);
        // oob.updateTally(oobLineCountForPrc(precompile));
        // mod.updateTally(modLinesComingFromOobCall(precompile));
      }
      case PRC_MODEXP -> {
        hub.updateTally(NB_ROWS_HUB_PRC_MODEXP);
        final MemoryRange memoryRange = new MemoryRange(0, 0, callData.size(), callData);
        final ModexpMetadata modexpMetadata = new ModexpMetadata(memoryRange);
        blakemodexp.updateTally(modexpMetadata.getNumberOfRowsForModexp());
        modexpEffectiveCall.updateTally(prcSuccess);
        modexpLargeCall.updateTally(modexpMetadata.largeModexp());
        // if (modexpMetadata.loadRawLeadingWord()) {
        //   final ExpCall modexpLogCallToExp = new ModexpLogExpCall(modexpMetadata);
        //   exp.call(modexpLogCallToExp);
        // }
        // oob.updateTally(oobLineCountForPrc(precompile));
        // mod.updateTally(modLinesComingFromOobCall(precompile));
      }
      case PRC_ECPAIRING -> {
        // trigger EcData to count the underlying EC operations
        if (callDataSize != 0 && callDataSize % TOTAL_SIZE_ECPAIRING_DATA_MIN == 0) {
          // Note: we can't know the id (and we don't care)
          ecdata.callEcData(0, precompile, frame.getInputData(), returnData);
        }
        hub.updateTally(NB_ROWS_HUB_PRC_ELLIPTIC_CURVE);
        // oob.updateTally(oobLineCountForPrc(precompile));
        // mod.updateTally(modLinesComingFromOobCall(precompile));
      }
      case PRC_BLAKE2F -> {
        blakeEffectiveCall.updateTally(true);
        hub.updateTally(NB_ROWS_HUB_PRC_BLAKE);
        // oob.updateTally(oobLineCountForPrc(PRC_BLAKE2F));
        blakemodexp.updateTally(numberOfRowsBlake());
        // TODO: still unchecked module for now: blakeRounds.updateTally();
      }
      case PRC_BLS_G1_ADD,
          PRC_BLS_G1_MSM,
          PRC_BLS_G2_ADD,
          PRC_BLS_G2_MSM,
          PRC_BLS_PAIRING_CHECK,
          PRC_BLS_MAP_FP_TO_G1,
          PRC_BLS_MAP_FP2_TO_G2,
          PRC_POINT_EVALUATION -> {
        if (validCallDataSize(precompile, callDataSize)) {
          blsdata.callBls(0, precompile, frame.getInputData(), returnData, prcSuccess);
        }
        hub.updateTally(NB_ROWS_HUB_PRC_ELLIPTIC_CURVE);
        // oob.updateTally(oobLineCountForPrc(precompile));
        // mod.updateTally(modLinesComingFromOobCall(precompile));
      }
      case PRC_P256_VERIFY -> {
        if (callDataSize == PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY) {
          ecdata.callEcData(0, precompile, frame.getInputData(), returnData);
        }
        hub.updateTally(NB_ROWS_HUB_PRC_ELLIPTIC_CURVE);
        // oob.updateTally(oobLineCountForPrc(precompile));
      }
      default -> throw new IllegalStateException("Unsupported precompile: " + precompile);
    }
  }

  /** When called, erase all tracing related to the bundle of all transactions since the last. */
  @Override
  public void popTransactionBundle() {
    for (Module m : checkedModules()) {
      m.popTransactionBundle();
    }
  }

  @Override
  public void commitTransactionBundle() {
    for (Module m : checkedModules()) {
      m.commitTransactionBundle();
    }
  }

  @Override
  public Map<String, Integer> getModulesLineCount() {
    final HashMap<String, Integer> modulesLineCount = HashMap.newHashMap(moduleToCount.size());

    for (Module m : checkedModules()) {
      modulesLineCount.put(m.moduleKey().toString(), m.lineCount() + m.spillage(trace));
    }
    for (Module m : uncheckedModules()) {
      modulesLineCount.put(m.moduleKey().toString(), 0);
    }
    return modulesLineCount;
  }

  @Override
  public List<Module> getModulesToCount() {
    return moduleToCount;
  }

  private boolean stackException(MessageFrame frame) {
    final OpCodeData opcode = opCodes.of(frame.getCurrentOperation().getOpcode());

    // Check for SUX / SOX, this is the only exception we check for
    final short stackSize = (short) frame.stackSize();
    final short deleted = (short) opcode.stackSettings().delta();
    // if we count WCP: final boolean underflow = wcp.callLT(stackSize, deleted);
    final boolean underflow = stackSize < deleted;
    if (underflow) {
      hub.updateTally(opcode.numberOfStackRows());
      return true;
    }
    final short heightNew = (short) (stackSize + opcode.stackSettings().alpha() - deleted);
    // if we count WCP: final boolean overflow = wcp.callGT(heightNew, MAX_STACK_SIZE);
    final boolean overflow = heightNew > MAX_STACK_SIZE;
    if (overflow) {
      hub.updateTally(opcode.numberOfStackRows());
      return true;
    }
    return false;
  }
}
