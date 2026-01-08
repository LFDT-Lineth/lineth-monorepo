/*
 * Copyright ConsenSys AG.
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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.TraceSubFragment;
import org.hyperledger.besu.evm.frame.MessageFrame;

/** This interface defines the API required to execute a call to the OOB module. */
public abstract class OobCall extends ModuleOperation implements TraceSubFragment {

  protected OobCall() {}

  public abstract Trace.Oob traceOob(Trace.Oob trace);

  public abstract void setInputs(Hub hub, MessageFrame frame);

  public abstract void setOutputs();

  public abstract boolean equals(Object o);

  public abstract int hashCode();

  @Override
  protected int computeLineCount() {
    return nRows();
  }

  /**
   * Default to one, some OOB inst can take more rows, depending on how corset expands the different
   * call
   */
  protected int nRows() {
    return 1;
  }
}
