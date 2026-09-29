package io.github.tomatool.mskiam;

import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Collectors;
import javax.security.auth.callback.CallbackHandler;
import javax.security.sasl.SaslServer;
import javax.security.sasl.SaslServerFactory;
import org.apache.kafka.common.errors.SaslAuthenticationException;

/**
 * The broker half of AWS_MSK_IAM, as far as a test needs it.
 *
 * <p>The client (aws-msk-iam-auth) sends one JSON payload signed with SigV4. MSK verifies the signature and then
 * checks the identity's IAM policy. Here the signature is not verified; the identity is. When
 * TOMATO_MSK_IAM_ALLOWED_ACCESS_KEY_IDS lists access key ids (tomato sets it to the role sessions its aws
 * resource issues), only those get in, standing in for "the role that has the kafka-cluster policy", and anyone
 * else gets MSK's answer, "Access denied". When it is empty, any well-formed payload gets in.
 */
public final class MskIamSaslServer implements SaslServer {
    public static final String MECHANISM = "AWS_MSK_IAM";

    private static final Pattern ACTION = Pattern.compile("\"action\"\\s*:\\s*\"kafka-cluster:Connect\"");
    private static final Pattern CREDENTIAL = Pattern.compile("\"x-amz-credential\"\\s*:\\s*\"([^/\"]+)/");

    private final Set<String> allowedAccessKeyIds;
    private boolean complete;
    private String authorizationId;

    MskIamSaslServer(Set<String> allowedAccessKeyIds) {
        this.allowedAccessKeyIds = allowedAccessKeyIds;
    }

    @Override
    public String getMechanismName() {
        return MECHANISM;
    }

    @Override
    public byte[] evaluateResponse(byte[] response) {
        String requestId = UUID.randomUUID().toString();
        String payload = new String(response, StandardCharsets.UTF_8);
        Matcher credential = CREDENTIAL.matcher(payload);
        if (!ACTION.matcher(payload).find() || !credential.find()) {
            log("rejected a payload that is not an AWS_MSK_IAM connect request");
            throw new SaslAuthenticationException("[" + requestId + "]: Invalid authentication payload");
        }
        String accessKeyId = credential.group(1);
        if (!allowedAccessKeyIds.isEmpty() && !allowedAccessKeyIds.contains(accessKeyId)) {
            log("denied " + accessKeyId);
            throw new SaslAuthenticationException("[" + requestId + "]: Access denied");
        }
        log("accepted " + accessKeyId);
        complete = true;
        authorizationId = accessKeyId;
        return ("{\"version\":\"2020_10_22\",\"request-id\":\"" + requestId + "\"}").getBytes(StandardCharsets.UTF_8);
    }

    @Override
    public boolean isComplete() {
        return complete;
    }

    @Override
    public String getAuthorizationID() {
        if (!complete) {
            throw new IllegalStateException("authentication is not complete");
        }
        return authorizationId;
    }

    @Override
    public byte[] unwrap(byte[] incoming, int offset, int len) {
        throw new IllegalStateException("AWS_MSK_IAM has no security layer");
    }

    @Override
    public byte[] wrap(byte[] outgoing, int offset, int len) {
        throw new IllegalStateException("AWS_MSK_IAM has no security layer");
    }

    @Override
    public Object getNegotiatedProperty(String propName) {
        if (!complete) {
            throw new IllegalStateException("authentication is not complete");
        }
        return null;
    }

    @Override
    public void dispose() {}

    private static void log(String message) {
        System.out.println("[tomato-msk-iam] " + message);
    }

    public static final class Factory implements SaslServerFactory {
        @Override
        public SaslServer createSaslServer(
                String mechanism, String protocol, String serverName, Map<String, ?> props, CallbackHandler handler) {
            if (!MECHANISM.equals(mechanism)) {
                return null;
            }
            String allowed = System.getenv().getOrDefault("TOMATO_MSK_IAM_ALLOWED_ACCESS_KEY_IDS", "");
            return new MskIamSaslServer(Arrays.stream(allowed.split(","))
                    .map(String::trim)
                    .filter(id -> !id.isEmpty())
                    .collect(Collectors.toSet()));
        }

        @Override
        public String[] getMechanismNames(Map<String, ?> props) {
            return new String[] {MECHANISM};
        }
    }
}
