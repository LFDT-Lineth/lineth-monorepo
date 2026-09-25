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

import org.gradle.api.GradleException
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.FileTree
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.file.SourceDirectorySet
import org.gradle.api.plugins.JavaPlugin
import org.gradle.api.plugins.JavaPluginExtension
import org.gradle.api.provider.Property
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.IgnoreEmptyDirectories
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.InputFile
import org.gradle.api.tasks.InputFiles
import org.gradle.api.tasks.Internal
import org.gradle.api.tasks.Optional
import org.gradle.api.tasks.OutputDirectory
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.SkipWhenEmpty
import org.gradle.api.tasks.SourceSet
import org.gradle.api.tasks.SourceSetContainer
import org.gradle.api.tasks.SourceTask
import org.gradle.api.tasks.TaskAction
import org.web3j.sokt.SolcInstance
import org.web3j.sokt.SolcRelease
import org.web3j.sokt.VersionResolver

import java.util.concurrent.TimeUnit

import static org.codehaus.groovy.runtime.StringGroovyMethods.capitalize

class YulExtension {
    static final NAME = "yul"
    String executable
    Project project
    String solcVersion
    String compilerJsonTemplatePath

    YulExtension(Project project) {
        this.project = project
    }
}

@CacheableTask
abstract class YulCompile extends SourceTask {
    /** Explicit solc binary; fingerprinted by content so a different compiler invalidates the cache. */
    @Optional
    @InputFile
    @PathSensitive(PathSensitivity.NONE)
    abstract RegularFileProperty getExecutable()

    /** solc version resolved through web3j-sokt when no explicit executable is configured. */
    @Optional
    @Input
    abstract Property<String> getSolcVersion()

    @InputFile
    @PathSensitive(PathSensitivity.NONE)
    abstract RegularFileProperty getCompilerJsonTemplate()

    /** Root of the generated resources; compiled contracts are written to its {@code yul/} subdirectory. */
    @OutputDirectory
    abstract DirectoryProperty getOutputDir()

    @Override
    @InputFiles
    @SkipWhenEmpty
    @IgnoreEmptyDirectories
    @PathSensitive(PathSensitivity.RELATIVE)
    FileTree getSource() {
        return super.getSource()
    }

    @TaskAction
    void compileYul() {
        String compilerExecutable = resolveCompilerExecutable()
        String compilerJsonTemplate = compilerJsonTemplate.get().asFile.getText()
        File yulOutputDir = new File(outputDir.get().asFile, YulExtension.NAME)
        // Start from a clean directory so outputs of removed contracts don't linger.
        yulOutputDir.deleteDir()
        yulOutputDir.mkdirs()

        def compilerProcessBuilder = new ProcessBuilder(
                compilerExecutable, "--pretty-json", "--standard-json", "-")
        .redirectErrorStream(true)

        for (def contract in source) {
            def compilerInput = compilerJsonTemplate
                    .replaceAll("%%YUL_FILE_NAME%%", contract.name)
                    .replaceAll("%%YUL_FILE_PATH%%", contract.absolutePath)

            def compilerProcess = compilerProcessBuilder.start()
            compilerProcess.getOutputStream().withCloseable {os ->
                os.newPrintWriter().withCloseable {writer ->
                    writer.println(compilerInput)
                    writer.flush()
                }
            }
            def output = compilerProcess.getText()
            boolean success = compilerProcess.waitFor(5, TimeUnit.SECONDS)
            if (!success) {
                throw new GradleException("Failed to compile ${contract}")
            }
            if (compilerProcess.exitValue() != 0) {
                throw new GradleException("Failed to compile ${contract}, solc exited with ${compilerProcess.exitValue()}:\n${output}")
            }
            def outputFile = new File(yulOutputDir, contract.name.replace(".yul", ".json"))
            outputFile.newPrintWriter("UTF-8").withCloseable {
                it.println(output)
                it.flush()
            }
        }
    }

    private String resolveCompilerExecutable() {
        if (executable.isPresent()) {
            return executable.get().asFile.absolutePath
        }
        if (!solcVersion.isPresent()) {
            throw new GradleException("Specify one of yul solcVersion or executable")
        }
        String version = solcVersion.get()
        SolcRelease resolvedVersion = new VersionResolver().getSolcReleases().stream().filter {
            it.getVersion() == version && it.isCompatibleWithOs()
        }.findAny().orElseThrow {
            return new GradleException("Failed to resolve Solidity version ${version}")
        }
        def compilerInstance = new SolcInstance(resolvedVersion, ".web3j", false)
        if (compilerInstance.installed() || !compilerInstance.installed() && compilerInstance.install()) {
            return compilerInstance.solcFile.getAbsolutePath()
        }
        throw new GradleException("Failed to install Solidity version ${version}")
    }
}


class YulPlugin implements Plugin<Project> {

    @Override
    void apply(Project target) {
        target.pluginManager.apply(JavaPlugin.class)
        target.extensions.create(YulExtension.NAME, YulExtension)

        JavaPluginExtension javaExtension = target.getExtensions().getByType(JavaPluginExtension.class)
        final SourceSetContainer sourceSets =  javaExtension.getSourceSets()
        sourceSets.configureEach { SourceSet sourceSet ->
            configureSourceSet(target, sourceSet)
        }

        target.afterEvaluate {
            sourceSets.configureEach { SourceSet sourceSet ->
                configureYulCompile(target, sourceSet)
            }
        }
    }

    /**
     * Add default source set for Yul.
     */
    private static void configureSourceSet(final Project project, final SourceSet sourceSet) {
        def yulSourceSet = getYulSourceSet(project, sourceSet)
        sourceSet.allJava.source(yulSourceSet)
        sourceSet.allSource.source(yulSourceSet)
    }

    private static SourceDirectorySet getYulSourceSet(final Project project, final SourceSet sourceSet) {
        def srcSetName = capitalize((CharSequence) sourceSet.name)
        final String sourceDirectoryDisplayName = srcSetName + " Yul Sources"
        def yulSourceSet = project.objects.sourceDirectorySet(YulExtension.NAME,  sourceDirectoryDisplayName)
        yulSourceSet.include("**/*.yul")
        def defaultSrcDir = new File(project.projectDir, "src/${sourceSet.name}/${YulExtension.NAME}")
        yulSourceSet.srcDirs(defaultSrcDir)
        return yulSourceSet
    }

    private static void configureYulCompile(final Project project, final SourceSet sourceSet) {
        def srcSetName = sourceSet.name == 'main' ? '' : capitalize((CharSequence) sourceSet.name)
        YulExtension yulExtension = project.extensions.getByType(YulExtension)
        def compileTask = project.tasks.register("compile${srcSetName}Yul", YulCompile) {
            it.source(getYulSourceSet(project, sourceSet))
            // Read lazily: the executable may be wired by another plugin's afterEvaluate (linea.solc-toolchain).
            it.executable.set(project.layout.file(project.provider {
                yulExtension.executable ? new File(yulExtension.executable) : null
            }))
            it.solcVersion.set(project.provider { yulExtension.solcVersion })
            it.compilerJsonTemplate.set(project.layout.projectDirectory.file(project.provider {
                yulExtension.compilerJsonTemplatePath
            }))
            it.outputDir.set(project.layout.buildDirectory.dir("generated/resources/yul/${sourceSet.name}"))
        }
        // Compiled contracts end up on the classpath as yul/<name>.json via processResources,
        // which also carries the task dependency (no manual build/jar wiring needed).
        sourceSet.resources.srcDir(compileTask.flatMap { it.outputDir })
    }
}
