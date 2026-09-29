# Builds tomato-msk-iam.jar, the AWS_MSK_IAM SASL server in src/. tomato embeds
# the committed jar and, for `auth: aws_msk_iam`, copies it into the kafka
# preset's apache/kafka container, so there is no image of its own.
#
#   make kafka-plugin    rebuild the jar after changing src/
#
# CI rebuilds it and fails when the committed jar differs, so the JDK is pinned
# and the jar's entries are written in a fixed order with a fixed date: the same
# sources always give the same bytes.
ARG KAFKA_IMAGE=apache/kafka:3.9.1

FROM ${KAFKA_IMAGE} AS kafka

FROM eclipse-temurin:21.0.12.1_1-jdk AS build
COPY --from=kafka /opt/kafka/libs/ /kafka-libs/
COPY src/ /src/
# --release 17: Kafka brokers run on Java 17 and later.
RUN mkdir /classes /out \
 && javac --release 17 -cp "/kafka-libs/*" -d /classes $(find /src -name '*.java' | sort) \
 && cd /classes \
 && jar --create --file /out/tomato-msk-iam.jar --date=2000-01-01T00:00:00Z $(find . -name '*.class' | sort)

FROM scratch AS jar
COPY --from=build /out/tomato-msk-iam.jar /
