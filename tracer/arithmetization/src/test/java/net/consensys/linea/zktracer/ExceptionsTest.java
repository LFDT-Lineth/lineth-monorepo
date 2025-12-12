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

package net.consensys.linea.zktracer;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class ExceptionsTest extends TracerTestBase {

  @Test
  public void testPrettyStringOf() {
    // Below a few non-meaningful examples of usage of prettyStringOf
    // NONE
    System.out.println("... but " + Exceptions.prettyStringOf(OpCode.CALL, (short) 0));

    // STACK_UNDERFLOW and OUT_OF_GAS_EXCEPTION
    System.out.println("... but " + Exceptions.prettyStringOf(OpCode.CALL, (short) 18));

    // all
    System.out.println("... but " + Exceptions.prettyStringOf(OpCode.CALL, (short) 3071));
  }
}
