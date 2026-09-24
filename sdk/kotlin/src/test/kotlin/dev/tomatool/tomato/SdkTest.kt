package dev.tomatool.tomato

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import kotlin.test.assertEquals

class HelpersTest {
    @Test
    fun `quotes step arguments and refuses embedded quotes`() {
        assertEquals("\"orders\"", q("orders"))
        assertEquals("\"201\"", q(201))
        assertThrows<IllegalArgumentException> { q("a\"b") }
    }

    @Test
    fun `builds tables from row maps`() {
        val table = toTable(listOf(mapOf("id" to "o-1", "amount" to 42), mapOf("id" to "o-2", "amount" to null)))
        assertEquals(listOf(listOf("id", "amount"), listOf("o-1", "42"), listOf("o-2", "")), table)
    }

    @Test
    fun `serialises docstring JSON`() {
        assertEquals("""{"id":"o-1"}""", toJson(mapOf("id" to "o-1")))
        assertEquals("""{"raw":true}""", toJson("""{"raw":true}"""))
    }
}
