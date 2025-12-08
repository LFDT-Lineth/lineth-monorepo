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

package net.consensys.linea.zktracer.container.stacked;

import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.zktracer.module.wcp.WcpOperation;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes32;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

class WcpOperationTests extends TracerTestBase {

  @Test
  void equals() {
    Bytes32 a = Bytes32.ZERO.copy().mutableCopy();
    EWord ew = EWord.ofHexString(a.toHexString());
    // Even though both are Bytes32, the equal method fails on them:
    // Assertions.assertTrue(ew.equals(a));

    WcpOperation wo1 = new WcpOperation(WcpOperation.LEQbv, a, a);
    WcpOperation wo2 = new WcpOperation(WcpOperation.LEQbv, ew, ew);
    Assertions.assertTrue(wo1.equals(wo2));
    Assertions.assertTrue(wo2.equals(wo1));
  }

  @Test
  void different() {
    Bytes32 a = Bytes32.ZERO.copy().mutableCopy();
    EWord ew = EWord.ofHexString(Bytes32.repeat((byte) 1).toHexString());

    WcpOperation wo1 = new WcpOperation(WcpOperation.LEQbv, a, a);
    WcpOperation wo2 = new WcpOperation(WcpOperation.LEQbv, ew, ew);
    Assertions.assertFalse(wo1.equals(wo2));
    Assertions.assertFalse(wo2.equals(wo1));
  }
}
