package dev.tomatool.tomato

import org.junit.jupiter.api.extension.BeforeAllCallback
import org.junit.jupiter.api.extension.BeforeEachCallback
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.extension.ExtensionContext
import org.junit.jupiter.api.extension.ParameterContext
import org.junit.jupiter.api.extension.ParameterResolver
import java.io.File
import java.nio.file.Files
import java.util.concurrent.TimeUnit

/**
 * Runs the test class against `tomato serve`: containers, the app and every
 * resource in [config] start once for the whole test run, and state is reset
 * before every test — the same guarantee as a Gherkin scenario.
 *
 * The tomato binary is `tomato` on the PATH, or the `tomato.bin` system
 * property / `TOMATO_BIN` environment variable.
 */
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
@ExtendWith(TomatoExtension::class)
annotation class TomatoTest(val config: String = "tomato.yml")

class TomatoExtension : BeforeAllCallback, BeforeEachCallback, ParameterResolver {

    override fun beforeAll(context: ExtensionContext) {
        server(context)
    }

    override fun beforeEach(context: ExtensionContext) {
        server(context).tomato.reset()
    }

    override fun supportsParameter(p: ParameterContext, e: ExtensionContext) = p.parameter.type == Tomato::class.java

    override fun resolveParameter(p: ParameterContext, e: ExtensionContext): Any = server(e).tomato

    private fun server(context: ExtensionContext): Server {
        val config = context.requiredTestClass.getAnnotation(TomatoTest::class.java)?.config ?: "tomato.yml"
        // One server per config for the whole run, stopped when JUnit finishes.
        val store = context.root.getStore(ExtensionContext.Namespace.create(TomatoExtension::class.java))
        return store.getOrComputeIfAbsent("server:$config", { Server.start(config) }, Server::class.java)
    }

    internal class Server private constructor(
        private val process: Process,
        val tomato: Tomato,
    ) : ExtensionContext.Store.CloseableResource {

        override fun close() {
            tomato.shutdown()
            if (!process.waitFor(30, TimeUnit.SECONDS)) process.destroy()
        }

        companion object {
            fun start(config: String): Server {
                val bin = System.getProperty("tomato.bin") ?: System.getenv("TOMATO_BIN") ?: "tomato"
                val ready = Files.createTempFile("tomato-serve", ".url").toFile().also { it.delete() }
                val log = File("build/tomato-serve.log").apply { parentFile.mkdirs() }
                val process = ProcessBuilder(bin, "serve", "-c", config, "--ready-file", ready.path, "--quiet")
                    .redirectErrorStream(true)
                    .redirectOutput(log)
                    .start()

                val deadline = System.nanoTime() + TimeUnit.MINUTES.toNanos(5)
                while (!ready.exists() || ready.length() == 0L) {
                    if (!process.isAlive) {
                        error("tomato serve exited with ${process.exitValue()}:\n${log.readText().takeLast(4000)}")
                    }
                    if (System.nanoTime() > deadline) {
                        process.destroy()
                        error("tomato serve was not ready after 5 minutes; see ${log.path}")
                    }
                    Thread.sleep(200)
                }
                return Server(process, Tomato(ready.readText().trim()))
            }
        }
    }
}
