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

package net.consensys.linea.zktracer.module.hub.fragment.common;

import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.*;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_EXEC;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.*;

import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;

@Accessors(fluent = true, chain = false)
@RequiredArgsConstructor
public final class CommonFragment implements TraceFragment {

  final CommonFragmentValues commonFragmentValues;
  private final int nonStackRowsCounter;
  private final boolean twoLineInstructionCounter;
  private final int mmuStamp;
  private final int mxpStamp;

  public CommonFragment(
      CommonFragmentValues commonValues,
      int stackLineCounter,
      int nonStackLineCounter,
      int mmuStamp,
      int mxpStamp) {
    this.commonFragmentValues = commonValues;
    this.twoLineInstructionCounter = stackLineCounter == 1;
    this.nonStackRowsCounter = nonStackLineCounter;
    this.mmuStamp = mmuStamp;
    this.mxpStamp = mxpStamp;
  }

  private boolean isUnexceptional() {
    return Exceptions.none(commonFragmentValues.exceptions);
  }

  public Trace.Hub trace(Trace.Hub trace) {
    final CallFrame frame = commonFragmentValues.callFrame;
    final boolean isExec = commonFragmentValues.hubProcessingPhase == TX_EXEC;
    trace
        .sysiTxnNumber(commonFragmentValues.sysiTransactionNumber)
        .userTxnNumber(commonFragmentValues.userTransactionNumber)
        .sysfTxnNumber(commonFragmentValues.sysfTransactionNumber)
        .blkNumber(commonFragmentValues.relBlockNumber)
        .sysi(commonFragmentValues.transactionProcessingType == SYSI)
        .user(commonFragmentValues.transactionProcessingType == USER)
        .sysf(commonFragmentValues.transactionProcessingType == SYSF)
        .txAuth(commonFragmentValues.hubProcessingPhase == TX_AUTH)
        .txWarm(commonFragmentValues.hubProcessingPhase == TX_WARM)
        .txSkip(commonFragmentValues.hubProcessingPhase == TX_SKIP)
        .txInit(commonFragmentValues.hubProcessingPhase == TX_INIT)
        .txExec(commonFragmentValues.hubProcessingPhase == TX_EXEC)
        .txFinl(commonFragmentValues.hubProcessingPhase == TX_FINL)
        .hubStamp(commonFragmentValues.hubStamp)
        .hubStampTransactionEnd(tx() == null ? 0 : tx().getHubStampTransactionEnd())
        .contextMayChange(commonFragmentValues.contextMayChange)
        .exceptionAhoy(Exceptions.any(commonFragmentValues.exceptions) && isExec)
        .logInfoStamp(commonFragmentValues.logStamp)
        .mmuStamp(mmuStamp)
        .mxpStamp(mxpStamp)
        // nontrivial dom / sub are traced in storage or account fragments only
        .contextNumber(isExec ? frame.contextNumber() : 0)
        .contextNumberNew(commonFragmentValues.contextNumberNew)
        .callerContextNumber(
            commonFragmentValues.callStack.getById(frame.parentId()).contextNumber())
        .contextWillRevert(frame.willRevert() && isExec)
        .contextGetsReverted(frame.getsReverted() && isExec)
        .contextSelfReverts(frame.selfReverts() && isExec)
        .contextRevertStamp(isExec ? frame.revertStamp() : 0)
        .codeFragmentIndex(commonFragmentValues.codeFragmentIndex)
        .programCounter(commonFragmentValues.pc)
        .programCounterNew(commonFragmentValues.pcNew)
        .height(isExec ? commonFragmentValues.height : 0)
        .heightNew(isExec ? commonFragmentValues.heightNew : 0)
        // peeking flags are traced in the respective fragments
        .gasExpected(Bytes.ofUnsignedLong(commonFragmentValues.gasExpected))
        .gasActual(Bytes.ofUnsignedLong(commonFragmentValues.gasActual))
        .gasCost(Bytes.ofUnsignedLong(commonFragmentValues.gasCostToTrace()))
        .gasNext(
            Bytes.ofUnsignedLong(isExec && isUnexceptional() ? commonFragmentValues.gasNext : 0))
        .refundCounter(commonFragmentValues.gasRefund)
        .refundCounterNew(commonFragmentValues.gasRefundNew)
        .twoLineInstruction(commonFragmentValues.TLI)
        .counterTli(twoLineInstructionCounter)
        .nonStackRows((short) commonFragmentValues.numberOfNonStackRows)
        .counterNsr((short) nonStackRowsCounter);
    return trace;
  }

  private TransactionProcessingMetadata tx() {
    return commonFragmentValues.txMetadata;
  }
}
