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

import org.gradle.api.DefaultTask
import org.gradle.api.file.FileSystemOperations
import org.gradle.api.file.ProjectLayout
import org.gradle.api.model.ObjectFactory
import org.gradle.api.provider.Property
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.InputFile
import org.gradle.api.tasks.Internal
import org.gradle.api.tasks.OutputDirectory
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction

import javax.inject.Inject

// Not cacheable: generation is cheap, only re-running it needlessly was costly.
abstract class RefTestGenerationTask extends DefaultTask {

  /** Directory holding the reference test JSON files, relative to the project directory. */
  @Internal
  abstract Property<String> getRefTests();

  @Internal
  abstract Property<String> getGeneratedRefTestsOutput();

  @Input
  abstract Property<String> getRefTestsSrcPath();

  @Input
  abstract Property<String> getRefTestNamePrefix();

  @Internal
  abstract Property<String> getRefTestTemplateFilePath();

  @Input
  abstract Property<String> getRefTestJsonParamsExcludedPath();

  @Input
  abstract Property<String> getRefTestJsonParamsDirectory();

  @Input
  abstract Property<String> getFailedModule();

  @Input
  abstract Property<String> getFailedConstraint();

  @Input
  abstract Property<String> getFailedTestsFilePath();

  @Inject
  abstract ProjectLayout getLayout()

  @Inject
  abstract ObjectFactory getObjects()

  @Inject
  abstract FileSystemOperations getFs()

  /**
   * The generated tests only embed the paths of the reference test files, not their content, so
   * the sorted relative paths are the input (avoids hashing the whole fixtures tree).
   */
  @Input
  List<String> getRefTestRelativePaths() {
    def dir = layout.projectDirectory.dir(refTests.get()).asFile
    refTestFiles().collect { dir.toPath().relativize(it.toPath()).toString().replace('\\', '/') }
  }

  @InputFile
  @PathSensitive(PathSensitivity.NONE)
  File getRefTestTemplateFile() {
    layout.projectDirectory.file(refTestTemplateFilePath.get()).asFile
  }

  @OutputDirectory
  File getGeneratedRefTestsOutputDir() {
    layout.projectDirectory.dir(generatedRefTestsOutput.get()).asFile
  }

  private List<File> refTestFiles() {
    objects.fileTree().from(layout.projectDirectory.dir(refTests.get())).files.sort()
  }

  @TaskAction
  def generateTests() {
    def refTestJsonParamsDirectory = getRefTestJsonParamsDirectory().get()
    def refTestsSrcPath = getRefTestsSrcPath().get()
    def generatedTestsDir = getGeneratedRefTestsOutputDir()
    def refTestNamePrefix = getRefTestNamePrefix().get()
    def excludedPath = getRefTestJsonParamsExcludedPath().get() // exclude test for test filling tool
    def failedTestsFilePath = getFailedTestsFilePath().get()
    def failedModule = getFailedModule().get()
    def failedConstraint = getFailedConstraint().get()

    // Delete directory with generated tests from previous run.
    fs.delete { it.delete(generatedTestsDir) }

    // Create directory to generate the tests before executing them.
    generatedTestsDir.mkdirs()

    def referenceTestTemplate = getRefTestTemplateFile().text

    // This is how many json files to include in each test file
    def fileSets = refTestFiles().collate(5)

    fileSets.eachWithIndex { fileSet, idx ->
      def paths = []
      fileSet.each { testJsonFile ->
        def parentFile = testJsonFile.getParentFile()
        def parentPathFile = parentFile.getPath().substring(parentFile.getPath().indexOf(refTestJsonParamsDirectory))
        if (!testJsonFile.getName().toString().startsWith(".") && !excludedPath.contains(parentPathFile)) {
          def pathFile = testJsonFile.getPath()
          paths << pathFile.substring(pathFile.indexOf(refTestJsonParamsDirectory)).replace('\\','/')
        }
      }

      def testFile = new File(generatedTestsDir, refTestNamePrefix + "_" + idx + ".java")
      def allPaths = '"' + paths.join('", "') + '"'

      def testFileContents = referenceTestTemplate
        .replaceAll("%%TESTS_FILE%%", allPaths)
        .replaceAll("%%TESTS_NAME%%", refTestNamePrefix + "_" + idx)
        .replaceAll("%%TESTS_SRC_PATH%%", refTestsSrcPath)
        .replaceAll("%%FAILED_TEST_FILE_PATH%%", failedTestsFilePath)
        .replaceAll("%%FAILED_MODULE%%", failedModule)
        .replaceAll("%%FAILED_CONSTRAINT%%", failedConstraint)
      testFile.newWriter().withWriter { w -> w << testFileContents }
    }
  }
}
