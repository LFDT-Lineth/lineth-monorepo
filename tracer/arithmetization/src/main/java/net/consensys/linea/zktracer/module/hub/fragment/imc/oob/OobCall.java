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

import java.util.ArrayList;
import java.util.List;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.TraceSubFragment;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.hyperledger.besu.evm.frame.MessageFrame;

/** This interface defines the API required to execute a call to the OOB module. */
public abstract class OobCall implements TraceSubFragment {

  public final List<OobExoCall> exoCalls;

  protected OobCall() {
    exoCalls = new ArrayList<>(ctMax());
  }

  public abstract Trace.Oob trace(Trace.Oob trace);

  public abstract void setInputData(MessageFrame frame, Hub hub);

  public abstract void callExoModulesAndSetOutputs(final Add add, final Mod mod, final Wcp wcp);

  public abstract int ctMax();

  public abstract boolean equals(Object o);

  public abstract int hashCode();
}
