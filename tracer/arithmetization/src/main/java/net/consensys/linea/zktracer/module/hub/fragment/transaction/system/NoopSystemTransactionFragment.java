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

package net.consensys.linea.zktracer.module.hub.fragment.transaction.system;

import static net.consensys.linea.zktracer.module.hub.fragment.transaction.system.SystemTransactionType.SYSF_NOOP;

import net.consensys.linea.zktracer.Trace;

public class NoopSystemTransactionFragment extends SystemTransactionFragment {

  public NoopSystemTransactionFragment() {
    super(SYSF_NOOP);
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    super.trace(trace);
    return trace.pTransactionNoop(true);
  }
}
