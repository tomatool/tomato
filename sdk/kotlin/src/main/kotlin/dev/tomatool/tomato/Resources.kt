package dev.tomatool.tomato

import kotlin.time.Duration

// Typed wrappers over tomato's steps. Each call builds the step text a feature
// file would contain and runs it, so behaviour (and error messages) are the
// same as a Gherkin run. Anything not wrapped here is still one `step()` away.

class HttpResource internal constructor(private val t: Tomato, private val name: String) {
    private val r = q(name)

    fun header(key: String, value: String) = apply { t.step("$r header ${q(key)} is ${q(value)}") }

    fun get(path: String) = send("GET", path)
    fun delete(path: String) = send("DELETE", path)
    fun post(path: String, json: Any) = send("POST", path, json)
    fun put(path: String, json: Any) = send("PUT", path, json)
    fun patch(path: String, json: Any) = send("PATCH", path, json)

    fun send(method: String, path: String, json: Any? = null): HttpResponseAssert {
        if (json == null) t.step("$r sends ${q(method)} to ${q(path)}")
        else t.step("$r sends ${q(method)} to ${q(path)} with json:", docString = toJson(json))
        return HttpResponseAssert(t, r)
    }
}

class HttpResponseAssert internal constructor(private val t: Tomato, private val r: String) {
    fun status(code: Int) = apply { t.step("$r response status is ${q(code)}") }

    /** The response JSON contains these fields (extra fields are ignored). Supports tomato's @matchers. */
    fun jsonContains(expected: Any) = apply { t.step("$r response json contains:", docString = toJson(expected)) }

    /** The response JSON is exactly this. */
    fun jsonMatches(expected: Any) = apply { t.step("$r response json matches:", docString = toJson(expected)) }

    fun json(path: String, value: Any) = apply { t.step("$r response json ${q(path)} is ${q(value)}") }

    fun header(key: String, value: String) = apply { t.step("$r response header ${q(key)} is ${q(value)}") }

    /** Stores a JSON field for later steps as `{{name}}`; returns its value. */
    fun save(path: String, variable: String): String {
        t.step("$r response json ${q(path)} saved as \"{{$variable}}\"")
        return t.variable(variable) ?: error("variable $variable was not stored")
    }
}

class DatabaseResource internal constructor(private val t: Tomato, private val name: String) {
    private val r = q(name)

    fun execute(sql: String) = apply { t.step("$r executes:", docString = sql) }

    fun table(table: String) = TableAssert(t, r, table)
}

class TableAssert internal constructor(private val t: Tomato, private val r: String, private val table: String) {
    /** Inserts rows. */
    fun insert(vararg rows: Map<String, Any?>) = apply {
        t.step("$r table ${q(table)} has values:", table = toTable(rows.toList()))
    }

    /** The table contains these rows (only the given columns are compared). */
    fun contains(vararg rows: Map<String, Any?>) = apply {
        t.step("$r table ${q(table)} contains:", table = toTable(rows.toList()))
    }

    fun hasRows(count: Int) = apply { t.step("$r table ${q(table)} has ${q(count)} rows") }

    fun isEmpty() = apply { t.step("$r table ${q(table)} is empty") }
}

class KafkaResource internal constructor(private val t: Tomato, private val name: String) {
    private val r = q(name)

    fun consume(topic: String) = apply { t.step("$r consumes from ${q(topic)}") }

    fun registerSchema(subject: String, file: String) =
        apply { t.step("$r registers schema for subject ${q(subject)} from file ${q(file)}") }

    fun publishJson(topic: String, value: Any, key: String? = null) = apply {
        if (key == null) t.step("$r publishes json to ${q(topic)}:", docString = toJson(value))
        else t.step("$r publishes json to ${q(topic)} with key ${q(key)}:", docString = toJson(value))
    }

    fun publishAvro(topic: String, value: Any, key: String? = null) = apply {
        if (key == null) t.step("$r publishes avro to ${q(topic)}:", docString = toJson(value))
        else t.step("$r publishes avro to ${q(topic)} with key ${q(key)}:", docString = toJson(value))
    }

    /** Waits for an Avro message on [topic] whose JSON form contains [expected]. */
    fun receivesAvro(topic: String, within: Duration, expected: Any) = apply {
        t.step("$r receives avro from ${q(topic)} within ${q(within.inWholeMilliseconds.toString() + "ms")}:", docString = toJson(expected))
    }

    fun lastMessageKey(key: String) = apply { t.step("$r last message has key ${q(key)}") }
}

class ShellResource internal constructor(private val t: Tomato, private val name: String) {
    private val r = q(name)

    fun run(script: String) = apply { t.step("$r runs:", docString = script) }
    fun exitCode(code: Int) = apply { t.step("$r exit code is ${q(code)}") }
    fun stdoutContains(text: String) = apply { t.step("$r stdout contains ${q(text)}") }
}
