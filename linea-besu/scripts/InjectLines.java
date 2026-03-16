/*
 * Copyright 2025 Consensys Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package scripts;

import java.io.*;
import java.nio.file.*;
import java.util.List;

public class InjectLines {

    private static final String INJECTION = """
        repositories {
            maven {
                name = "cloudsmith"
                url = "https://api.cloudsmith.io/maven/consensys/linea-besu/"
                credentials {
                    username = System.getenv("CLOUDSMITH_USER");
                    password = System.getenv("CLOUDSMITH_API_KEY");
                }
            }
        }
    """;

    public static void main(String[] args) {
        if (args.length != 1) {
            System.err.println("Usage: java scripts.InjectLines <file>");
            System.exit(1);
        }

        injectLines(args[0]);
    }

    private static void injectLines(String filePath) {
        try {
            Path path = Paths.get(filePath);
            List<String> lines = Files.readAllLines(path);
            boolean injected = false;

            try (BufferedWriter writer = Files.newBufferedWriter(path)) {
                for (String line : lines) {
                    writer.write(line);
                    writer.newLine();
                    if (!injected && line.contains("publishing {")) {
                        writer.write(INJECTION);
                        writer.newLine();
                        injected = true;
                    }
                }
            }

            if (!injected) {
                System.err.println(
                    "Injection failed: could not find 'publishing {' in " + filePath
                        + "; Cloudsmith repositories block was not added.");
                System.exit(1);
            }
            System.out.println("Lines injected successfully into " + filePath);
        } catch (IOException e) {
            System.err.println("Injection failed for " + filePath + ": " + e.getMessage());
            e.printStackTrace();
            System.exit(1);
        }
    }
}

