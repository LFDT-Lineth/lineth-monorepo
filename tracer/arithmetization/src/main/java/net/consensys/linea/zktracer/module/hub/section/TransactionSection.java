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

package net.consensys.linea.zktracer.module.hub.section;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;

public class TransactionSection extends TraceSection {

  public TransactionSection(Hub hub) {
    // 2 = 1 + 1     (stack, transaction)
    // 3 = 1 + 1 + 1 (stack, transaction, context)
    super(hub, Exceptions.none(hub.pch().exceptions()) ? (short) 2 : (short) 3);

    this.addStackAndFragments(hub, hub.txStack().current().userTransactionFragment());
  }
}
