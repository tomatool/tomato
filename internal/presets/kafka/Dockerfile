# The kafka preset image: apache/kafka with an AWS_MSK_IAM SASL server on its
# classpath, so a listener can behave like MSK's IAM port. The release workflow
# publishes it as ghcr.io/tomatool/tomato-kafka:<version>; a development build of
# tomato builds it from the copy embedded in the binary.
ARG KAFKA_IMAGE=apache/kafka:3.9.1

FROM ${KAFKA_IMAGE} AS kafka

FROM eclipse-temurin:21-jdk AS build
COPY --from=kafka /opt/kafka/libs/ /kafka-libs/
COPY src/ /src/
RUN mkdir /classes \
 && javac --release 17 -cp "/kafka-libs/*" -d /classes $(find /src -name '*.java') \
 && jar --create --file /tomato-msk-iam.jar -C /classes .

FROM ${KAFKA_IMAGE}
COPY --from=build /tomato-msk-iam.jar /opt/kafka/libs/tomato-msk-iam.jar
