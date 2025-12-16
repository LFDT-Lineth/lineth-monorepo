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
package net.consensys.linea.zktracer.lt;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.ObjectWriter;
import java.io.IOException;
import java.io.OutputStream;
import java.nio.ByteBuffer;
import java.util.List;
import java.util.Map;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;

/** Provides a basic API for writing an LT trace file. */
public abstract class LtFile {
  protected final Header header;

  public LtFile(Header header) {
    this.header = header;
  }

  /**
   * Access columns for writing where each column is indexed by its corresponding register id.
   *
   * @return
   */
  protected abstract Trace.Column[] columns();

  /**
   * Generate a binary encoding of the given trace.
   *
   * @return
   */
  public abstract void write(OutputStream out) throws IOException;

  /**
   * Construct the binary representation of a trace.
   *
   * @param header Header information to use
   * @param trace
   * @param modules
   * @return
   */
  public static LtFile of(Header header, Trace trace, List<Module> modules) {
    // Initialise trace file
    final LtFile ltf =
        switch (header.majorVersion()) {
          case 1 -> new LtFileV1(header, trace, modules);
          case 2 -> new LtFileV2(header, trace, modules);
          default ->
              throw new IllegalArgumentException(
                  "unsupported lt trace file version (v"
                      + header.majorVersion()
                      + "."
                      + header.minorVersion()
                      + ")");
        };
    // Open trace for writing
    trace.open(ltf.columns());
    // Commit each module
    for (Module m : modules) {
      m.commit(trace);
    }
    // Convert to bytes
    return ltf;
  }

  public record Header(int majorVersion, int minorVersion, Map<String, Object> metadata) {
    /**
     * Construct trace file header containing the given metadata bytes.
     *
     * @return bytes making up the header.
     */
    public byte[] toBytes() throws IOException {
      byte[] metadataBytes = getMetadataBytes(metadata);
      //
      ByteBuffer buffer = ByteBuffer.allocate(16 + metadataBytes.length);
      // File identifier
      buffer.put(new byte[] {'z', 'k', 't', 'r', 'a', 'c', 'e', 'r'});
      // Major version
      buffer.putShort((short) majorVersion);
      // Minor version
      buffer.putShort((short) minorVersion);
      // Metadata length
      buffer.putInt(metadataBytes.length);
      // Metadata
      buffer.put(metadataBytes);
      // Done
      return buffer.array();
    }

    /** Object writer is used for generating JSON byte strings. */
    private static final ObjectWriter objectWriter = new ObjectMapper().writer();

    public static byte[] getMetadataBytes(Map<String, Object> metadata) throws IOException {
      return objectWriter.writeValueAsBytes(metadata);
    }
  }
}
