import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.tasks.JavaExec
import org.gradle.api.tasks.SourceSetContainer

/**
 * Wires the config-documentation check/generate tasks for an application module.
 *
 * Creates a dedicated {@code configDocs} source set (compiled against {@code main} plus the
 * {@code jvm-libs:linea:config-docs} engine, and kept out of the production jar), and registers
 * {@code checkConfigDocs} and {@code generateConfigDocs} tasks that run the engine's generic entry
 * points against the app's {@code ConfigDocsSpec}.
 *
 * Apply after the Kotlin/application conventions plugin and configure the spec:
 * <pre>
 * plugins { id 'net.consensys.zkevm.kotlin-application-conventions'; id 'linea.config-docs' }
 * configDocs { spec = "com.example.MyConfigDocsSpec" }
 * </pre>
 */
class ConfigDocsPlugin implements Plugin<Project> {
  @Override
  void apply(Project project) {
    def extension = project.extensions.create('configDocs', ConfigDocsExtension)

    def sourceSets = project.extensions.getByType(SourceSetContainer)
    def main = sourceSets.getByName('main')
    def configDocs = sourceSets.create('configDocs')
    configDocs.compileClasspath += main.output + main.compileClasspath
    configDocs.runtimeClasspath += main.output + main.runtimeClasspath

    // Build-time only tooling engine; kept off the production runtime classpath.
    project.dependencies.add(
        'configDocsImplementation',
        project.project(':jvm-libs:linea:config-docs'))

    def specProvider = project.provider {
      def spec = extension.spec
      if (!spec) {
        throw new IllegalStateException(
            "configDocs.spec must be set to the fully-qualified ConfigDocsSpec class name")
      }
      spec
    }

    def checkConfigDocs = project.tasks.register('checkConfigDocs', JavaExec) {
      it.group = 'verification'
      it.description = 'Verifies that every config key is documented.'
      it.classpath = configDocs.runtimeClasspath
      it.mainClass.set('linea.config.docs.ConfigDocsCheckMain')
      it.argumentProviders.add({ [specProvider.get()] } as org.gradle.process.CommandLineArgumentProvider)
    }

    // Enforce documentation completeness as part of `check` (and therefore CI's buildNeeded).
    project.tasks.matching { it.name == 'check' }.configureEach { it.dependsOn(checkConfigDocs) }

    project.tasks.register('generateConfigDocs', JavaExec) {
      it.group = 'documentation'
      it.description = 'Generates the config JSON schema snapshot and Markdown reference.'
      it.classpath = configDocs.runtimeClasspath
      it.mainClass.set('linea.config.docs.ConfigDocsGenerateMain')
      it.workingDir = project.rootProject.projectDir
      it.argumentProviders.add({ [specProvider.get()] } as org.gradle.process.CommandLineArgumentProvider)
    }
  }
}