package com.example.orders

import org.apache.avro.Schema
import org.apache.avro.generic.GenericData
import org.apache.avro.generic.GenericRecord
import org.springframework.beans.factory.annotation.Value
import org.springframework.http.HttpStatus
import org.springframework.jdbc.core.JdbcTemplate
import org.springframework.kafka.annotation.KafkaListener
import org.springframework.kafka.core.KafkaTemplate
import org.springframework.stereotype.Service
import org.springframework.transaction.annotation.Transactional
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RestController
import org.springframework.web.server.ResponseStatusException

data class CreateOrder(val id: String, val amount: Long, val currency: String)

data class Order(val id: String, val amount: Long, val currency: String, val status: String)

private fun schema(name: String): Schema =
    Schema.Parser().parse(OrderService::class.java.getResourceAsStream("/avro/$name.avsc"))

@Service
class OrderService(
    private val jdbc: JdbcTemplate,
    private val kafka: KafkaTemplate<String, Any>,
    @Value("\${orders.topics.created}") private val createdTopic: String,
    @Value("\${orders.topics.paid}") private val paidTopic: String,
) {
    private val orderCreated = schema("order_created")
    private val orderPaid = schema("order_paid")

    @Transactional
    fun create(req: CreateOrder): Order {
        val known = jdbc.queryForObject(
            "SELECT count(*) FROM currencies WHERE code = ?", Int::class.java, req.currency,
        )
        if (known == 0) {
            throw ResponseStatusException(HttpStatus.UNPROCESSABLE_ENTITY, "unknown currency ${req.currency}")
        }
        val inserted = jdbc.update(
            "INSERT INTO orders (id, amount, currency, status) VALUES (?, ?, ?, 'CREATED') ON CONFLICT DO NOTHING",
            req.id, req.amount, req.currency,
        )
        if (inserted == 0) {
            throw ResponseStatusException(HttpStatus.CONFLICT, "order ${req.id} already exists")
        }

        val event = GenericData.Record(orderCreated).apply {
            put("id", req.id)
            put("amount", req.amount)
            put("currency", req.currency)
        }
        kafka.send(createdTopic, req.id, event).get()
        return Order(req.id, req.amount, req.currency, "CREATED")
    }

    fun find(id: String): Order? =
        jdbc.query("SELECT id, amount, currency, status FROM orders WHERE id = ?", { rs, _ ->
            Order(rs.getString("id"), rs.getLong("amount"), rs.getString("currency"), rs.getString("status"))
        }, id).firstOrNull()

    @KafkaListener(topics = ["\${orders.topics.payments}"])
    fun onPayment(payment: GenericRecord) {
        val orderId = payment.get("orderId").toString()
        val updated = jdbc.update("UPDATE orders SET status = 'PAID' WHERE id = ?", orderId)
        if (updated == 0) return

        val event = GenericData.Record(orderPaid).apply {
            put("id", orderId)
            put("paymentId", payment.get("paymentId").toString())
        }
        kafka.send(paidTopic, orderId, event).get()
    }
}

@RestController
class OrderController(private val orders: OrderService) {
    @PostMapping("/orders")
    fun create(@RequestBody req: CreateOrder): org.springframework.http.ResponseEntity<Order> =
        org.springframework.http.ResponseEntity.status(HttpStatus.CREATED).body(orders.create(req))

    @GetMapping("/orders/{id}")
    fun get(@PathVariable id: String): Order =
        orders.find(id) ?: throw ResponseStatusException(HttpStatus.NOT_FOUND, "order $id not found")
}
