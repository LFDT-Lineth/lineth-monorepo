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

package net.consensys.linea.testing;

import java.util.List;
import java.util.function.BiConsumer;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes32;

/**
 * Dynamic test case data structure.
 *
 * @param name name of the test case
 * @param arguments arguments for the test case
 * @param customAssertions optional custom assertions per test case
 */
public record DynamicTestCase(
    String name, List<OpcodeCall> arguments, BiConsumer<OpCode, List<Bytes32>> customAssertions) {}
