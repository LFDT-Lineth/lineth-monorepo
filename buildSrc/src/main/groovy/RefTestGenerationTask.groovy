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
import org.gradle.api.provider.Property
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.TaskAction

abstract class RefTestGenerationTask extends DefaultTask {

  @Input
  abstract Property<String> getRefTests();

  @Input
  abstract Property<String> getGeneratedRefTestsOutput();

  @Input
  abstract Property<String> getRefTestsSrcPath();

  @Input
  abstract Property<String> getRefTestNamePrefix();

  @Input
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

  @TaskAction
  def generateTests() {
    def refTests = project.fileTree(getRefTests().get())
    def refTestTemplateFile = project.file(getRefTestTemplateFilePath().get())
    def refTestJsonParamsDirectory = getRefTestJsonParamsDirectory().get()
    def refTestsSrcPath = getRefTestsSrcPath().get()
    def generatedTestsFilePath = getGeneratedRefTestsOutput().get()
    def refTestNamePrefix = getRefTestNamePrefix().get()
    def excludedPath = getRefTestJsonParamsExcludedPath().get() // exclude test for test filling tool
    def failedTestsFilePath = getFailedTestsFilePath().get()
    def failedModule = getFailedModule().get()
    def failedConstraint = getFailedConstraint().get()

    // Delete directory with generated tests from previous run.
    project.delete(generatedTestsFilePath)

    // Create directory to generate the tests before executing them.
    project.mkdir(generatedTestsFilePath)

    def referenceTestTemplate = refTestTemplateFile.text

    // This is how many json files to include in each test file
    def fileSets = refTests.getFiles().sort().collate(5)

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

      def testFile = project.file(generatedTestsFilePath + "/" + refTestNamePrefix + "_" + idx + ".java")
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
