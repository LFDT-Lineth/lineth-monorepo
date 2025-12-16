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

package net.consensys.linea.zktracer.module.gas;

import static net.consensys.linea.zktracer.module.ModuleName.GAS;

import java.math.BigInteger;
import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.PostOpcodeDefer;
import net.consensys.linea.zktracer.module.hub.fragment.common.CommonFragmentValues;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.module.hub.signals.TracedException;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.operation.Operation;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class Gas implements OperationSetModule<GasOperation>, PostOpcodeDefer {
  /** A list of the operations to trace */
  @Getter
  private final ModuleOperationStackedSet<GasOperation> operations =
      new ModuleOperationStackedSet<>();

  private CommonFragmentValues commonValues;
  private GasParameters gasParameters;

  @Override
  public ModuleName moduleKey() {
    return GAS;
  }

  public void call(Hub hub, CommonFragmentValues commonValues) {
    this.commonValues = commonValues;
    gasParameters = new GasParameters();
    hub.defers().scheduleForPostExecution(this);
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.gas().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.gas().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (GasOperation gasOperation : operations.sortOperations(new GasOperation.GasComparator())) {
      gasOperation.trace(trace.gas());
    }
  }

  @Override
  public void resolvePostExecution(
      Hub hub, MessageFrame frame, Operation.OperationResult operationResult) {
    gasParameters.gasActual(BigInteger.valueOf(commonValues.gasActual));
    gasParameters.gasCost(BigInteger.valueOf(commonValues.gasCostToTrace()));
    gasParameters.xahoy(Exceptions.any(commonValues.exceptions));
    gasParameters.oogx(commonValues.tracedException() == TracedException.OUT_OF_GAS_EXCEPTION);
    this.operations.add(new GasOperation(gasParameters));
  }
}
