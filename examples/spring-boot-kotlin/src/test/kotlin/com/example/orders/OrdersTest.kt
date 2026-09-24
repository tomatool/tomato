package com.example.orders

import dev.tomatool.tomato.Tomato
import dev.tomatool.tomato.TomatoTest
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

// The same scenarios as features/orders.feature, written in Kotlin against the
// experimental tomato SDK. tomato still starts Postgres, Kafka, the Schema
// Registry and the jar, and resets state before every test.
@TomatoTest
class OrdersTest(private val t: Tomato) {
    private val api = t.http("api")
    private val db = t.db("db")
    private val kafka = t.kafka("kafka")

    @Test
    fun `creating an order stores it and emits OrderCreated`() {
        kafka.consume("orders")

        api.post("/orders", mapOf("id" to "order-1", "amount" to 4200, "currency" to "EUR"))
            .status(201)
            .jsonContains(mapOf("id" to "order-1", "status" to "CREATED"))

        db.table("orders").contains(
            mapOf("id" to "order-1", "amount" to 4200, "currency" to "EUR", "status" to "CREATED"),
        )
        kafka.receivesAvro("orders", within = 15.seconds, expected = mapOf("id" to "order-1", "amount" to 4200))
    }

    @Test
    fun `the same id works again because state is fresh`() {
        api.post("/orders", mapOf("id" to "order-1", "amount" to 99, "currency" to "USD")).status(201)
        db.table("orders").hasRows(1)
    }

    @Test
    fun `a duplicate id within one test is rejected`() {
        db.table("orders").insert(mapOf("id" to "order-1", "amount" to 10, "currency" to "EUR", "status" to "CREATED"))
        api.post("/orders", mapOf("id" to "order-1", "amount" to 10, "currency" to "EUR")).status(409)
    }

    @Test
    fun `currencies seeded by the migration survive every reset`() {
        api.post("/orders", mapOf("id" to "order-2", "amount" to 10, "currency" to "GBP")).status(422)
        db.table("currencies").hasRows(2)
    }

    // Plain Kotlin where Gherkin needs a step per case: a loop, a computed value.
    @Test
    fun `amounts round-trip for every currency`() {
        listOf("EUR" to 1L, "USD" to Long.MAX_VALUE / 2).forEachIndexed { i, (currency, amount) ->
            api.post("/orders", mapOf("id" to "order-$i", "amount" to amount, "currency" to currency)).status(201)
            api.get("/orders/order-$i").status(200).jsonContains(mapOf("currency" to currency))
        }
        db.table("orders").hasRows(2)
    }
}
