package dev.tomatool.tomato

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.fasterxml.jackson.module.kotlin.readValue
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Duration

/** A step failed. The message is tomato's own failure message. */
class TomatoStepFailed(val step: String, message: String) : AssertionError("$step\n  $message")

/**
 * A connection to `tomato serve`. Every tomato step is available through
 * [step]; the typed resources ([http], [db], [kafka], ...) build those steps
 * for you so tests read like Kotlin instead of step text.
 */
class Tomato(val baseUrl: String) {
    private val client = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build()
    private val json = jacksonObjectMapper()

    /** Runs one step, exactly as it would appear in a feature file (no Given/When/Then). */
    fun step(text: String, docString: String? = null, table: List<List<String>>? = null) {
        val body = buildMap<String, Any> {
            put("step", text)
            docString?.let { put("docString", it.trimIndent()) }
            table?.let { put("table", it) }
        }
        val (status, response) = post("/v1/steps/run", body)
        if (status != 200) throw TomatoStepFailed(text, response["error"]?.toString() ?: "HTTP $status")
    }

    /** Resets every resource and clears variables: what tomato does before each scenario. */
    fun reset() {
        val (status, response) = post("/v1/reset", emptyMap<String, Any>())
        check(status == 200) { "tomato reset failed: ${response["error"]}" }
    }

    /** A variable a step stored, e.g. `response json "id" saved as "{{order_id}}"`. */
    fun variable(name: String): String? {
        val response = client.send(
            HttpRequest.newBuilder(URI.create("$baseUrl/v1/variables/$name")).GET().build(),
            HttpResponse.BodyHandlers.ofString(),
        )
        if (response.statusCode() == 404) return null
        return json.readValue<Map<String, Any?>>(response.body())["value"]?.toString()
    }

    fun http(name: String) = HttpResource(this, name)
    fun db(name: String) = DatabaseResource(this, name)
    fun kafka(name: String) = KafkaResource(this, name)
    fun shell(name: String) = ShellResource(this, name)

    internal fun shutdown() {
        runCatching { post("/v1/shutdown", emptyMap<String, Any>()) }
    }

    private fun post(path: String, body: Any): Pair<Int, Map<String, Any?>> {
        val request = HttpRequest.newBuilder(URI.create(baseUrl + path))
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(json.writeValueAsString(body)))
            .timeout(Duration.ofMinutes(2))
            .build()
        val response = client.send(request, HttpResponse.BodyHandlers.ofString())
        val parsed: Map<String, Any?> = if (response.body().isBlank()) emptyMap() else json.readValue(response.body())
        return response.statusCode() to parsed
    }
}

/** Quotes a value for step text. Step arguments are `"..."` and can't contain `"`. */
internal fun q(value: Any): String {
    val s = value.toString()
    require('"' !in s) { "step arguments can't contain a double quote: $s" }
    return "\"$s\""
}

private val mapper = jacksonObjectMapper()

/** JSON for a docstring, from a map, a data class, or a raw JSON string. */
internal fun toJson(value: Any): String = if (value is String) value else mapper.writeValueAsString(value)

/** A table from a list of rows given as maps (column -> value); the first row's keys are the header. */
internal fun toTable(rows: List<Map<String, Any?>>): List<List<String>> {
    require(rows.isNotEmpty()) { "a table needs at least one row" }
    val columns = rows.first().keys.toList()
    return listOf(columns) + rows.map { row -> columns.map { row[it]?.toString() ?: "" } }
}
