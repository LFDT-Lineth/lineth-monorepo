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

package net.consensys.linea.zktracer.module.rlpUtils;

import lombok.EqualsAndHashCode;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;

@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public abstract class RlpUtilsCall extends ModuleOperation {
  public static final short NB_ROWS_RLP_UTILS = 1;

  protected RlpUtilsCall() {}

  protected abstract void compute();

  public abstract void traceRlpTxn(
      Trace.Rlptxn trace,
      GenericTracedValue tracedValues,
      boolean lt,
      boolean lx,
      boolean updateTracedValue,
      int ct);

  protected void trace(Trace.Rlputils trace) {
    traceMacro(trace);
  }

  protected abstract void traceMacro(Trace.Rlputils trace);

  protected abstract short instruction();

  protected abstract short compareTo(RlpUtilsCall other);

  @Override
  protected int computeLineCount() {
    return NB_ROWS_RLP_UTILS;
  }
}
