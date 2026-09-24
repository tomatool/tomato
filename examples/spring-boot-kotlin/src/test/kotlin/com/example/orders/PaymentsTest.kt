package com.example.orders

import dev.tomatool.tomato.Tomato
import dev.tomatool.tomato.TomatoTest
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

// features/payments.feature in Kotlin: tomato publishes Avro, the app's
// @KafkaListener consumes it.
@TomatoTest
class PaymentsTest(private val t: Tomato) {
    private val api = t.http("api")
    private val db = t.db("db")
    private val kafka = t.kafka("kafka")

    @BeforeEach
    fun background() {
        kafka.registerSchema("payments-value", "src/main/resources/avro/payment_received.avsc")
        kafka.consume("orders-paid")
    }

    @Test
    fun `a payment marks the order paid`() {
        db.table("orders").insert(mapOf("id" to "order-7", "amount" to 4200, "currency" to "EUR", "status" to "CREATED"))

        kafka.publishAvro("payments", key = "order-7", value = mapOf("paymentId" to "pay-1", "orderId" to "order-7", "amount" to 4200))

        kafka.receivesAvro("orders-paid", within = 20.seconds, expected = mapOf("id" to "order-7", "paymentId" to "pay-1"))
        db.table("orders").contains(mapOf("id" to "order-7", "status" to "PAID"))
        api.get("/orders/order-7").jsonContains(mapOf("status" to "PAID"))
    }

    @Test
    fun `a payment for an unknown order changes nothing`() {
        kafka.publishAvro("payments", mapOf("paymentId" to "pay-2", "orderId" to "missing", "amount" to 1))
        api.get("/orders/missing").status(404)
        db.table("orders").isEmpty()
    }
}
