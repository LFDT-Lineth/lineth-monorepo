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

package net.consensys.linea.zktracer.module.ecdata.ecpairing;

import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.UnitTestWatcher;
import org.junit.jupiter.api.extension.ExtendWith;

@Getter
@Setter
@ExtendWith(UnitTestWatcher.class)
public class EcPairingArgumentsSingleton {
  private static EcPairingArgumentsSingleton instance;
  private String arguments;

  private EcPairingArgumentsSingleton() {
    // private constructor to prevent instantiation
  }

  public static EcPairingArgumentsSingleton getInstance() {
    if (instance == null) {
      instance = new EcPairingArgumentsSingleton();
    }
    return instance;
  }
}
