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

package net.consensys.linea.zktracer.module.hub.fragment;

import static net.consensys.linea.zktracer.types.AddressUtils.hiPart;
import static net.consensys.linea.zktracer.types.AddressUtils.loPart;

import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.Either;
import net.consensys.linea.zktracer.types.MemoryRange;
import org.hyperledger.besu.datatypes.Address;

/**
 * Represents a context fragment in the trace. Ideally one would provide a {@link CallFrame}
 * directly. But when initializing a new context we may only provide a {@code callFrameReference}.
 * Note that the {@code returnDataRange} may evolve during the lifetime of a {@link CallFrame}. As
 * such we provide its current value.
 */
@Getter
@Setter
@Accessors(fluent = true)
@AllArgsConstructor
public class ContextFragment implements TraceFragment {
  private final Hub hub;
  private final Either<Integer, Integer> callFrameReference; // left: id, right: context number
  private final MemoryRange returnDataRange;
  private final boolean updateReturnData;

  /** The following set of methods are used to read without modifying a context. */
  public static ContextFragment readContextData(Hub hub, CallFrame callFrame) {
    return new ContextFragment(
        hub, Either.left(callFrame.id()), callFrame.returnDataRange().snapshot(), false);
  }

  public static ContextFragment readZeroContextData(final Hub hub) {
    return readContextData(hub, CallFrame.EMPTY);
  }

  public static ContextFragment readCurrentContextData(final Hub hub) {
    return readContextData(hub, hub.currentFrame());
  }

  /** The following set of methods are used to update the return data of a context. */
  public static ContextFragment updateReturnData(
      final Hub hub, CallFrame callFrame, final MemoryRange returnDataRange) {

    callFrame.returnDataRange(returnDataRange);
    return new ContextFragment(hub, Either.left(callFrame.id()), returnDataRange.snapshot(), true);
  }

  public static ContextFragment executionProvidesReturnData(final Hub hub) {
    return updateReturnData(
        hub, hub.callStack().parentCallFrame(), hub.currentFrame().outputDataRange());
  }

  public static ContextFragment executionProvidesEmptyReturnData(final Hub hub) {
    return updateReturnData(
        hub,
        hub.callStack().parentCallFrame(),
        new MemoryRange(hub.currentFrame().contextNumber()));
  }

  public static ContextFragment nonExecutionProvidesEmptyReturnData(final Hub hub) {
    return updateReturnData(hub, hub.currentFrame(), new MemoryRange(hub.newChildContextNumber()));
  }

  /** Initialization of a new execution context */
  public static ContextFragment initializeExecutionContext(final Hub hub) {
    return new ContextFragment(
        hub, Either.right(hub.newChildContextNumber()), MemoryRange.EMPTY, false);
  }

  /** returns the account address, (stored in pContextAccountAddress) = aADDR in spec */
  public Address getAccountAddress() {
    return getCallFrame().accountAddress();
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    final CallFrame callFrame = getCallFrame();

    final Address address = callFrame.accountAddress();
    final Address codeAddress = callFrame.byteCodeAddress();
    final Address callerAddress = callFrame.callerAddress();

    return trace
        .peekAtContext(true)
        .pContextContextNumber(callFrame.contextNumber())
        .pContextCallStackDepth((short) callFrame.depth())
        .pContextIsRoot(callFrame.isRoot())
        .pContextIsStatic(callFrame.type().isStatic())
        .pContextAccountAddressHi(hiPart(address))
        .pContextAccountAddressLo(loPart(address))
        .pContextAccountDeploymentNumber(callFrame.accountDeploymentNumber())
        .pContextByteCodeAddressHi(hiPart(codeAddress))
        .pContextByteCodeAddressLo(loPart(codeAddress))
        .pContextByteCodeDeploymentNumber(callFrame.byteCodeDeploymentNumber())
        .pContextByteCodeDeploymentStatus(callFrame.isDeployment() ? 1 : 0)
        .pContextByteCodeCodeFragmentIndex(callFrame.getCodeFragmentIndex(hub))
        .pContextCallerAddressHi(hiPart(callerAddress))
        .pContextCallerAddressLo(loPart(callerAddress))
        .pContextCallValue(callFrame.value())
        .pContextCallDataContextNumber(callFrame.callDataRange().contextNumber())
        .pContextCallDataOffset(callFrame.callDataRange().offset())
        .pContextCallDataSize(callFrame.callDataRange().size())
        .pContextReturnAtOffset(callFrame.returnAtRange().offset())
        .pContextReturnAtCapacity(callFrame.returnAtRange().size())
        .pContextUpdate(updateReturnData)
        .pContextReturnDataContextNumber(returnDataRange.contextNumber())
        .pContextReturnDataOffset(returnDataRange.offset())
        .pContextReturnDataSize(returnDataRange.size());
  }

  private CallFrame getCallFrame() {
    return this.callFrameReference.map(
        hub.callStack()::getById, hub.callStack()::getByContextNumber);
  }
}
