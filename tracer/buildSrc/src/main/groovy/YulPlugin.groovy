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

import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.file.SourceDirectorySet
import org.gradle.api.plugins.JavaPlugin
import org.gradle.api.plugins.JavaPluginExtension
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.SourceSet
import org.gradle.api.tasks.SourceSetContainer
import org.gradle.api.tasks.SourceTask
import org.gradle.api.tasks.TaskAction
import org.web3j.sokt.SolcInstance
import org.web3j.sokt.SolcRelease
import org.web3j.sokt.VersionResolver

import java.nio.file.Path
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
class YulCompile extends SourceTask {
    @Input
    String outputDir

    @TaskAction
    void compileYul() {
        YulExtension yulExtension = project.getExtensions().getByName(YulExtension.NAME) as YulExtension
        String compilerExecutable = getCompilerExecutable(yulExtension)
        String compilerJsonTemplate = project.file(yulExtension.compilerJsonTemplatePath).getText()
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
                throw new Exception("Failed to compile ${contract}")
            }
            def outputFile = Path.of(outputDir).resolve(contract.name.replace(".yul", ".json")).toFile()
            outputFile.newPrintWriter("UTF-8").withCloseable {
                it.println(output)
                it.flush()
            }
        }
    }

    private static String getCompilerExecutable (final YulExtension yulExtension) {
        String compilerExecutable = yulExtension.executable
        if (compilerExecutable == null) {
            if (yulExtension.solcVersion == null) {
                println(yulExtension.getProperties())
                throw new Exception("Specify one of yul solcVersion or executable")
            }
            SolcRelease resolvedVersion = new VersionResolver().getSolcReleases().stream().filter {
                it.getVersion() == yulExtension.solcVersion && it.isCompatibleWithOs()
            }.findAny().orElseThrow {
                return new Exception("Failed to resolve Solidity version ${yulExtension.solcVersion}")
            }
            def compilerInstance = new SolcInstance(resolvedVersion, ".web3j", false)
            if (compilerInstance.installed() || !compilerInstance.installed() && compilerInstance.install()) {
                compilerExecutable = compilerInstance.solcFile.getAbsolutePath()
            }
        }
        return compilerExecutable
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
        def compileTask = project.tasks.register("compile${srcSetName}Yul", YulCompile)
        compileTask.configure {

            it.source(getYulSourceSet(project, sourceSet))
            def outDir = project
                .getLayout()
                .getBuildDirectory()
                .dir("resources/${sourceSet.name}/${YulExtension.NAME}").get()
            it.getOutputs().dir(outDir)
            project.delete(outDir)
            project.mkdir(outDir)
            it.outputDir = outDir.asFile.absolutePath
        }
        project.getTasks().named('build').configure {
            it.dependsOn(compileTask)
        }
        project.getTasks().named('jar').configure {
            it.dependsOn(compileTask)
        }
    }
}
