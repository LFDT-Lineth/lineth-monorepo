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
package net.consensys.linea.zktracer.module.hub.section.halt.selfdestruct;

import static net.consensys.linea.zktracer.module.hub.fragment.scenario.SelfdestructScenarioFragment.SelfdestructScenario.*;

import net.consensys.linea.zktracer.module.hub.Hub;
import org.hyperledger.besu.evm.frame.MessageFrame;

public class LondonSelfdestructSection extends SelfdestructSection {

  public LondonSelfdestructSection(Hub hub, MessageFrame frame) {
    super(hub, frame);
  }

  public boolean accountFragmentWiping() {
    // In London, the account fragment is always wiped
    return true;
  }

  public boolean softAccountWiping() {
    return false;
  }
}
