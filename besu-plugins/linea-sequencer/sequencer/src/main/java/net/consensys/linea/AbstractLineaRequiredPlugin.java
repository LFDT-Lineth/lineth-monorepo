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

package net.consensys.linea;

import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;

/**
 * Linea plugins extending this class will halt startup of Besu in case of exception during
 * registration.
 *
 * <p>If that's NOT desired, the plugin should implement {@link BesuPlugin} directly.
 */
@Slf4j
public abstract class AbstractLineaRequiredPlugin extends AbstractLineaSharedPrivateOptionsPlugin {

  @Override
  public void register(final ServiceManager serviceManager) {
    super.register(serviceManager);
    try {
      log.info("Registering Linea plugin {}", this.getClass().getName());

      doRegister(serviceManager);

    } catch (Exception e) {
      log.error("Halting Besu startup: exception in plugin registration: ", e);
      e.printStackTrace();
      // System.exit will cause besu to exit
      System.exit(1);
    }
  }

  /**
   * Linea plugins need to implement this method. Called by {@link BesuPlugin} register method
   *
   * @param serviceManager the ServiceManager to be used.
   */
  public abstract void doRegister(final ServiceManager serviceManager);

  @Override
  public void start() {
    super.start();
    try {
      log.info("Starting Linea plugin {}", this.getClass().getName());

      doStart();

    } catch (Exception e) {
      log.error("Halting Besu startup: exception in plugin startup: ", e);
      e.printStackTrace();
      // System.exit will cause besu to exit
      System.exit(1);
    }
  }

  /** Linea plugins can implement this method. Called by {@link BesuPlugin} start method */
  public abstract void doStart();
}
