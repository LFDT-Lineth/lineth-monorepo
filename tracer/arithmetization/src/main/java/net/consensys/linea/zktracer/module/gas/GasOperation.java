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

import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import java.util.Comparator;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class GasOperation extends ModuleOperation {

  public static final short NB_ROWS_GAS = 1;

  @EqualsAndHashCode.Include @Getter GasParameters gasParameters;

  public GasOperation(GasParameters gasParameters) {
    this.gasParameters = gasParameters;
  }

  private boolean compareGasActualAndGasCost() {
    return !gasParameters.xahoy() || gasParameters.oogx();
  }

  @Override
  protected int computeLineCount() {
    return NB_ROWS_GAS;
  }

  public void trace(Trace.Gas trace) {
    trace
        .gasActual(bigIntegerToBytes(gasParameters.gasActual()))
        .gasCost(bigIntegerToBytes(gasParameters.gasCost()))
        .xahoy(gasParameters.xahoy())
        .oogx(gasParameters.oogx())
        .validateRow();
  }

  public static class GasComparator implements Comparator<GasOperation> {
    @Override
    public int compare(GasOperation o1, GasOperation o2) {
      return o1.gasParameters().compareTo(o2.gasParameters());
    }
  }
}
