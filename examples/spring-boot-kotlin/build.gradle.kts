plugins {
    id("org.springframework.boot") version "3.3.4"
    id("io.spring.dependency-management") version "1.1.6"
    kotlin("jvm") version "2.0.20"
    kotlin("plugin.spring") version "2.0.20"
    jacoco
}

group = "com.example"
version = "0.0.1"

java {
    toolchain { languageVersion = JavaLanguageVersion.of(21) }
}

repositories {
    mavenCentral()
    maven("https://packages.confluent.io/maven/")
}

// The JaCoCo agent jar, attached to the app while tomato drives it.
val jacocoRuntimeAgent: Configuration by configurations.creating

dependencies {
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("org.springframework.boot:spring-boot-starter-jdbc")
    implementation("org.springframework.kafka:spring-kafka")
    implementation("com.fasterxml.jackson.module:jackson-module-kotlin")
    implementation("org.jetbrains.kotlin:kotlin-reflect")
    implementation("org.flywaydb:flyway-core")
    implementation("org.flywaydb:flyway-database-postgresql")
    implementation("io.confluent:kafka-avro-serializer:7.6.1")
    implementation("org.apache.avro:avro:1.11.3")
    runtimeOnly("org.postgresql:postgresql")

    jacocoRuntimeAgent("org.jacoco:org.jacoco.agent:0.8.12:runtime")
}

kotlin {
    compilerOptions { freeCompilerArgs.addAll("-Xjsr305=strict") }
}

tasks.bootJar { archiveFileName = "orders.jar" }
tasks.jar { enabled = false }

// Copies the agent to build/jacoco/jacocoagent.jar for tomato.yml's app.command.
val copyJacocoAgent by tasks.registering(Copy::class) {
    from(jacocoRuntimeAgent) { rename { "jacocoagent.jar" } }
    into(layout.buildDirectory.dir("jacoco"))
}
tasks.bootJar { finalizedBy(copyJacocoAgent) }

// HTML/XML coverage from the tomato run: ./gradlew tomatoCoverage
val tomatoCoverage by tasks.registering(JacocoReport::class) {
    executionData(layout.buildDirectory.file("jacoco/tomato.exec"))
    sourceSets(sourceSets["main"])
    reports {
        html.required = true
        xml.required = true
        html.outputLocation = layout.buildDirectory.dir("reports/tomato-coverage")
    }
}
