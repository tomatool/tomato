@kafka @glue
Feature: Kafka Avro in the AWS Glue wire format
  With schema_registry format glue, tomato writes and reads the Glue wire format
  (header version, compression, schema version UUID, Avro) and serves the Glue
  Schema Registry API the app's SerDe fetches schemas from.

  Scenario: Round-trip an Avro record in the Glue format
    Given "glue-events" registers schema for subject "glue-orders" from file "tests/testdata/order.avsc"
    And "glue-events" consumes from "glue-orders"
    When "glue-events" publishes avro to "glue-orders" with key "user-1":
      """
      {"id": "order-1", "amount": 4200, "currency": "EUR", "note": null}
      """
    Then "glue-events" receives avro from "glue-orders" within "10s":
      """
      {"id": "order-1", "amount": 4200}
      """
    And "glue-events" last message has key "user-1"
    And "glue-events" last message avro matches:
      """
      {"id": "order-1", "amount": 4200, "currency": "EUR", "note": null}
      """

  Scenario: The app is pointed at tomato's Glue registry
    When "app-client" sends "GET" to "/env?name=AWS_ENDPOINT_URL_GLUE"
    Then "app-client" response json "value" is "http://127.0.0.1:9994"

  Scenario: The app fetches a registered schema the way its SerDe does
    Given "glue-events" registers schema for subject "glue-orders" from file "tests/testdata/order.avsc"
    And "app-client" header "X-Amz-Target" is "AWSGlue.GetSchemaVersion"
    When "app-client" sends "POST" to "http://127.0.0.1:9994/" with json:
      """
      {"SchemaId": {"SchemaName": "glue-orders", "RegistryName": "portfolio"}, "SchemaVersionNumber": {"LatestVersion": true}}
      """
    Then "app-client" response status is "200"
    And "app-client" response json "DataFormat" is "AVRO"
    And "app-client" response json "SchemaArn" is "arn:aws:glue:eu-central-1:000000000000:schema/portfolio/glue-orders"
    And "app-client" response json "SchemaVersionId" is uuid
    And "app-client" response json "SchemaDefinition" exists
