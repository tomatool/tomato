// EXPERIMENTAL Kotlin SDK for tomato: write tomato tests as JUnit 5 tests in
// Kotlin instead of Gherkin. It drives `tomato serve`, so every resource and
// step tomato has is available, and state is reset before every test.
plugins {
    kotlin("jvm") version "2.0.20"
    `java-library`
}

group = "dev.tomatool"
version = "0.1.0-SNAPSHOT"

java {
    toolchain { languageVersion = JavaLanguageVersion.of(21) }
}

repositories { mavenCentral() }

dependencies {
    api("org.junit.jupiter:junit-jupiter-api:5.11.0")
    implementation("com.fasterxml.jackson.module:jackson-module-kotlin:2.17.2")

    testImplementation("org.junit.jupiter:junit-jupiter:5.11.0")
    testImplementation(kotlin("test"))
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test { useJUnitPlatform() }
