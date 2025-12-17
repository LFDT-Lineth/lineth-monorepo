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
package net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.Hub.newIdentifierFromStamp;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileScenario.*;
import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;

import java.util.ArrayList;
import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.*;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag;
import net.consensys.linea.zktracer.module.hub.section.call.CallSection;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.MemoryRange;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.frame.MessageFrame;

/** Note: {@link PrecompileSubsection}'s are created at child context entry by the call section */
@RequiredArgsConstructor
@Getter
@Accessors(fluent = true)
public abstract class PrecompileSubsection
    implements ContextEntryDefer, ContextExitDefer, ContextReEntryDefer, PostRollbackDefer {

  public final CallSection callSection;
  public MemoryRange returnDataRange;

  // gas parameters
  long callerGas;
  long calleeGas;
  long returnGas;

  // success bit
  boolean callSuccess;

  // special fragments
  public final List<TraceFragment> fragments;
  public final PrecompileScenarioFragment precompileScenarioFragment;
  public final ImcFragment firstImcFragment;

  /**
   * Default creator specifying the max number of rows the precompile processing subsection can
   * contain.
   */
  public PrecompileSubsection(final Hub hub, final CallSection callSection) {
    this.callSection = callSection;
    fragments = new ArrayList<>(maxNumberOfLines());

    hub.defers().scheduleForContextEntry(this); // gas & input data, ...
    hub.defers().scheduleForContextExit(this, hub.callStack().futureId());
    hub.defers().scheduleForContextReEntry(this, hub.currentFrame()); // success bit & return data

    final PrecompileFlag precompileFlag =
        addressToPrecompileFlag(callSection.precompileAddress.orElseThrow());

    precompileScenarioFragment =
        new PrecompileScenarioFragment(this, PRC_SUCCESS_WONT_REVERT, precompileFlag);
    fragments.add(precompileScenarioFragment);

    firstImcFragment = ImcFragment.empty(hub);
    fragments.add(firstImcFragment);
  }

  protected short maxNumberOfLines() {
    return 0;
  }

  @Override
  public void resolveUponContextEntry(Hub hub, MessageFrame frame) {
    callerGas = hub.callStack().parentCallFrame().frame().getRemainingGas();
    calleeGas = frame.getRemainingGas();
  }

  public void resolveUponContextExit(Hub hub, CallFrame callFrame) {
    returnGas = callFrame.frame().getRemainingGas();
  }

  @Override
  public void resolveAtContextReEntry(Hub hub, CallFrame callFrame) {
    callSuccess = bytesToBoolean(callFrame.frame().getStackItem(0));
    setReturnDataRange(callFrame.frame(), callSuccess);

    if (callSuccess) {
      hub.defers().scheduleForPostRollback(this, callFrame);
    }

    final CallFrame returnerFrame = hub.callStack().getByContextNumber(returnDataContextNumber());
    returnerFrame.outputDataRange(returnDataRange);
  }

  public void sanityCheck() {
    if (callSuccess) {
      checkArgument(
          precompileScenarioFragment.scenario.isSuccess(),
          "precompile scenario %s not success scenario",
          precompileScenarioFragment.scenario());
    } else {
      checkArgument(
          precompileScenarioFragment.scenario.isFailure(),
          "precompile scenario %s not failure scenario",
          precompileScenarioFragment.scenario());
    }
  }

  @Override
  public void resolveUponRollback(Hub hub, MessageFrame messageFrame, CallFrame callFrame) {

    // only successful PRC calls should enter here
    checkArgument(
        precompileScenarioFragment.scenario() == PRC_SUCCESS_WONT_REVERT,
        "precompile scenario %s incompatible with being rolled back");

    precompileScenarioFragment.scenario(PRC_SUCCESS_WILL_REVERT);
  }

  /** Our arithmetization distinguishes between {@link Address#MODEXP} and other precompiles. */
  private void setReturnDataRange(MessageFrame frame, boolean callSuccess) {

    // failed PRC_CALL
    if (!callSuccess) {
      returnDataRange = new MemoryRange(returnDataContextNumber());
      return;
    }

    // successful PRC_CALL to any precompile other than MODEXP
    if (flag() != PRC_MODEXP) {
      returnDataRange =
          new MemoryRange(
              returnDataContextNumber(), 0, frame.getReturnData().size(), frame.getReturnData());
      return;
    }

    // successful PRC_CALL to MODEXP
    final ModexpSubsection modexpSubsection = (ModexpSubsection) this;
    final int mbs = modexpSubsection.modexpMetadata.mbsInt();
    final int maxInputSize = modexpSubsection.modexpMetadata.getMaxInputSize();
    final Bytes returnData = frame.getReturnData();
    checkState(
        0 <= mbs && mbs <= maxInputSize,
        "MODEXP PrecompileSubsection: invalid mbs: %s not in range [0,%s]",
        mbs,
        maxInputSize);
    checkState(
        returnData.size() == mbs,
        "MODEXP PrecompileSubsection: return data size %s does not agree with mbs %s",
        returnData.size(),
        mbs);
    final Bytes leftPaddedReturnData = leftPadTo(returnData, maxInputSize);

    returnDataRange =
        new MemoryRange(returnDataContextNumber(), maxInputSize - mbs, mbs, leftPaddedReturnData);
  }

  public int exoModuleOperationId() {
    return newIdentifierFromStamp(callSection.hubStamp());
  }

  public int returnDataContextNumber() {
    return exoModuleOperationId();
  }

  public PrecompileFlag flag() {
    return precompileScenarioFragment.flag;
  }

  public void setScenario(PrecompileScenarioFragment.PrecompileScenario scenario) {
    precompileScenarioFragment.scenario(scenario);
  }

  public MemoryRange getCallDataRange() {
    return callSection.getCallDataRange();
  }

  public long callDataOffset() {
    return getCallDataRange().offset();
  }

  public long callDataSize() {
    return getCallDataRange().size();
  }

  public MemoryRange getReturnAtRange() {
    return callSection.getReturnAtRange();
  }

  public long returnAtOffset() {
    return getReturnAtRange().offset();
  }

  public long returnAtCapacity() {
    return getReturnAtRange().size();
  }

  public long returnDataOffset() {
    return returnDataRange().offset();
  }

  public long returnDataSize() {
    return returnDataRange().size();
  }

  public Bytes rawCallerMemory() {
    return getCallDataRange().isEmpty()
        ? getReturnAtRange().getRawData()
        : getCallDataRange().getRawData();
  }

  public Bytes extractCallData() {
    return callSection.getCallDataRange().extract();
  }

  public Bytes extractReturnData() {
    return returnDataRange.extract();
  }
}
