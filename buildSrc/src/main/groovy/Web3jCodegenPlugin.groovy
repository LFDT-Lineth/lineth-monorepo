import org.gradle.api.DefaultTask
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.artifacts.type.ArtifactTypeDefinition
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.ProjectLayout
import org.gradle.api.model.ObjectFactory
import org.gradle.api.plugins.JavaPlugin
import org.gradle.api.provider.MapProperty
import org.gradle.api.provider.Property
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.OutputDirectory
import org.gradle.api.tasks.TaskAction
import org.web3j.codegen.SolidityFunctionWrapperGenerator

import javax.inject.Inject

@CacheableTask
abstract class GenerateContractWrappersTask extends DefaultTask {
  @Input
  abstract MapProperty<String, String> getContracts()

  @Input
  abstract Property<String> getContractsPackage()

  @OutputDirectory
  abstract DirectoryProperty getGeneratedClassesDir()

  @TaskAction
  void action() {
    File outputDir = generatedClassesDir.get().asFile
    logger.lifecycle("Generating contract wrappers in ${outputDir.absolutePath}")

    contracts.get().each { abiFile, contractName ->
      logger.lifecycle("Generating contract $contractName $abiFile")
      String[] params = [
        '--abiFile', abiFile,
        '--outputDir', outputDir.absolutePath,
        '--package', contractsPackage.get(),
        '--contractName', contractName
      ]
      SolidityFunctionWrapperGenerator.main(params)
    }
  }
}

class Web3jCodegenExtension {
  final MapProperty<String, String> contracts
  final Property<String> contractsPackage
  final DirectoryProperty generatedClassesDir

  @Inject
  Web3jCodegenExtension(ObjectFactory objects, ProjectLayout layout) {
    contracts = objects.mapProperty(String, String)
    contractsPackage = objects.property(String).convention("")
    generatedClassesDir = objects.directoryProperty()
      .convention(layout.buildDirectory.dir("generated/sources/web3j/main/java"))
  }
}

class Web3jCodegenPlugin implements Plugin<Project> {
  static final String TASK_NAME = "generateWeb3jWrappers"

  @Override
  void apply(Project project) {
    Web3jCodegenExtension extension = project.extensions.create("web3jContractWrappers", Web3jCodegenExtension)

    project.tasks.register(TASK_NAME, GenerateContractWrappersTask) {
      group = "Code Generation"
      description = "Creates Web3J contract wrappers from ABIs files."
      contracts.set(extension.contracts)
      contractsPackage.set(extension.contractsPackage)
      generatedClassesDir.set(extension.generatedClassesDir)
    }

    project.plugins.withType(JavaPlugin) {
      // Compile generated sources in a dedicated source set so that -Werror can be
      // suppressed for generated code without affecting hand-written sources in main.
      project.sourceSets {
        generatedWeb3j {
          java {
            srcDir extension.generatedClassesDir
          }
          // inherit main's compile classpath (compileOnly, api, implementation)
          compileClasspath += project.sourceSets.main.compileClasspath
        }
      }

      def generatedCompileTask = project.tasks.named("compileGeneratedWeb3jJava")
      def generatedClassesDirs = project.sourceSets.generatedWeb3j.output.classesDirs

      generatedCompileTask.configure {
        dependsOn project.tasks.named(TASK_NAME)
        // Generated sources are not held to -Werror.
        options.compilerArgs.remove('-Werror')
      }

      // Register the generated classes directory in the 'classes' secondary variant of
      // apiElements and runtimeElements so that consuming project() dependencies resolve
      // them directly (compile-avoidance path) without sharing compileJava's output dir.
      ["apiElements", "runtimeElements"].each { configName ->
        project.configurations.named(configName).configure { config ->
          config.outgoing.variants.named("classes") { variant ->
            generatedClassesDirs.each { dir ->
              variant.artifact(dir) {
                type = ArtifactTypeDefinition.JVM_CLASS_DIRECTORY
                builtBy generatedCompileTask
              }
            }
          }
        }
      }

      // Include generated compiled classes in the jar artifact.
      // dependsOn is inferred: classesDirs carries builtBy(compileGeneratedWeb3jJava).
      project.tasks.named("jar").configure {
        from generatedClassesDirs
      }
    }
  }
}
