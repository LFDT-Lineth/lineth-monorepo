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

import groovy.io.FileType
import org.gradle.api.DefaultTask
import org.gradle.api.GradleException
import org.gradle.api.file.ProjectLayout
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.InputFiles
import org.gradle.api.tasks.OutputFile
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction

import javax.inject.Inject

abstract class CheckSpdxHeader extends DefaultTask {
    private String rootPath
    private String spdxHeader
    private String filesRegex
    private String excludeRegex

    @Input
    String getRootPath() {
        return rootPath
    }

    void setRootPath(final String rootPath) {
        this.rootPath = rootPath
    }

    @Input
    String getSpdxHeader() {
        return spdxHeader
    }

    void setSpdxHeader(final String spdxHeader) {
        this.spdxHeader = spdxHeader
    }

    @Input
    String getFilesRegex() {
        return filesRegex
    }

    void setFilesRegex(final String filesRegex) {
        this.filesRegex = filesRegex
    }

    @Input
    String getExcludeRegex() {
        return excludeRegex
    }

    void setExcludeRegex(final String excludeRegex) {
        this.excludeRegex = excludeRegex
    }

    @Inject
    abstract ProjectLayout getLayout()

    /** Files to check, so the task is up-to-date while none of them changes. */
    @InputFiles
    @PathSensitive(PathSensitivity.RELATIVE)
    List<File> getCheckedFiles() {
        def files = []
        new File(rootPath).traverse(
                type: FileType.FILES,
                nameFilter: ~/${filesRegex}/,
                excludeFilter: ~/${excludeRegex}/
        ) { f -> files.add(f) }
        return files.sort()
    }

    /** Marker written on success; a task without outputs is never up-to-date. */
    @OutputFile
    File getResultFile() {
        layout.buildDirectory.file("tmp/${name}/result.txt").get().asFile
    }

    @TaskAction
    void checkHeaders() {
        def filesWithoutHeader = getCheckedFiles().findAll { !it.getText().contains(spdxHeader) }

        if (!filesWithoutHeader.isEmpty()) {
            throw new GradleException("Files without headers: " + filesWithoutHeader.join('\n'))
        }
        getResultFile().text = "OK\n"
    }
}