package com.example.orders

import org.apache.kafka.clients.admin.NewTopic
import org.springframework.beans.factory.annotation.Value
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.kafka.config.TopicBuilder

// The app owns its topics, as it would in production. tomato only reads and
// writes them.
@Configuration
class Topics {
    @Bean
    fun createdTopic(@Value("\${orders.topics.created}") name: String): NewTopic = TopicBuilder.name(name).build()

    @Bean
    fun paymentsTopic(@Value("\${orders.topics.payments}") name: String): NewTopic = TopicBuilder.name(name).build()

    @Bean
    fun paidTopic(@Value("\${orders.topics.paid}") name: String): NewTopic = TopicBuilder.name(name).build()
}
