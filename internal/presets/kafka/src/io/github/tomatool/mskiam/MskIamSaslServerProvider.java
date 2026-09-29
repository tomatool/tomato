package io.github.tomatool.mskiam;

import java.security.Provider;
import java.security.Security;

/** Makes {@link MskIamSaslServer} what {@code Sasl.createSaslServer("AWS_MSK_IAM", ...)} returns inside the broker. */
public final class MskIamSaslServerProvider extends Provider {
    private static final String NAME = "tomato AWS_MSK_IAM";

    private MskIamSaslServerProvider() {
        super(NAME, "1.0", "AWS_MSK_IAM SASL server for tests");
        put("SaslServerFactory." + MskIamSaslServer.MECHANISM, MskIamSaslServer.Factory.class.getName());
    }

    public static synchronized void initialize() {
        if (Security.getProvider(NAME) == null) {
            Security.addProvider(new MskIamSaslServerProvider());
        }
    }
}
