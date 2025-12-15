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

package net.consensys.linea.zktracer.module.trm;

import static net.consensys.linea.zktracer.types.AddressUtils.highPart;
import static net.consensys.linea.zktracer.types.AddressUtils.isPrecompile;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.datatypes.Address;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class TrmOperation extends ModuleOperation {
  private final Fork fork;
  @EqualsAndHashCode.Include @Getter private final EWord rawAddress;

  public TrmOperation(Fork fork, EWord rawAddress) {
    this.fork = fork;
    this.rawAddress = rawAddress;
  }

  void trace(Trace.Trm trace) {
    final Address trmAddress = rawAddress.toAddress();
    final boolean isPrec = isPrecompile(fork, trmAddress);
    final long trmAddrHi = highPart(trmAddress);

    trace.rawAddress(rawAddress).addressHi(trmAddrHi).isPrecompile(isPrec).validateRow();
  }

  @Override
  protected int computeLineCount() {
    return 1;
  }
}
