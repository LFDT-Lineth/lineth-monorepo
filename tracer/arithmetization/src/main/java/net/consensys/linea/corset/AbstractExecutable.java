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
package net.consensys.linea.corset;

import java.io.IOException;
import java.nio.charset.Charset;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.apache.commons.io.IOUtils;

public class AbstractExecutable {
  protected record Outcome(int exitcode, String output, boolean timeout) {}

  /**
   * Execute a system command using a given command-line, producing an exitc-code and the sysout and
   * syserr streams. This operation can also timeout.
   *
   * @param timeout The timeout to use (in seconds).
   * @param commands The command-line to use for execution.
   * @return An outcome holding all the key information.
   * @throws IOException If an IO error of some kind arises.
   * @throws InterruptedException If the process is interrupted somehow.
   */
  protected Outcome exec(int timeout, List<String> commands)
      throws IOException, InterruptedException {
    return exec(timeout, commands.toArray(new String[commands.size()]));
  }

  /**
   * Execute a system command using a given command-line, producing an exitc-code and the sysout and
   * syserr streams. This operation can also timeout.
   *
   * @param timeout The timeout to use (in seconds).
   * @param commands The command-line to use for execution.
   * @return An outcome holding all the key information.
   * @throws IOException If an IO error of some kind arises.
   * @throws InterruptedException If the process is interrupted somehow.
   */
  protected Outcome exec(int timeout, String... commands) throws IOException, InterruptedException {
    // ===================================================
    // Construct the process
    // ===================================================
    ProcessBuilder builder =
        new ProcessBuilder(commands)
            .redirectInput(ProcessBuilder.Redirect.INHERIT)
            .redirectErrorStream(true);
    Process child = builder.start();
    try {
      // Read output from child process.  Note that this will include both the STDOUT and STDERR
      // since (for simplicity) the latter is redirected above.
      String output = IOUtils.toString(child.getInputStream(), Charset.defaultCharset());
      // Second, read the result whilst checking for a timeout
      boolean success = child.waitFor(timeout, TimeUnit.SECONDS);
      // Report everything back
      return new Outcome(child.exitValue(), output, !success);
    } finally {
      // make sure child process is destroyed.
      child.destroy();
    }
  }

  /**
   * Attempt to run an executable. This basically just checks it executes as expected and produces
   * some output.
   *
   * @param binary Binary executable to try
   * @return Flag indicating whether executing the given binary was successful or not.
   */
  protected boolean isExecutable(String... commands) {
    try {
      Outcome outcome = exec(5, commands);
      // Check process executed correctly, and produced some output.
      return outcome.exitcode() == 0 && !outcome.output().isEmpty();
    } catch (Throwable e) {
      return false;
    }
  }
}
