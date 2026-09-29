package io.github.tomatool.mskiam;

import java.util.Map;
import javax.security.auth.Subject;
import javax.security.auth.callback.CallbackHandler;
import javax.security.auth.spi.LoginModule;

/**
 * The login module an AWS_MSK_IAM listener names in its sasl.jaas.config. Kafka loads it when the listener starts,
 * and loading it registers the AWS_MSK_IAM SASL server, the way Kafka's own PlainLoginModule registers PLAIN. It
 * authenticates nothing itself.
 */
public class MskIamLoginModule implements LoginModule {
    static {
        MskIamSaslServerProvider.initialize();
    }

    @Override
    public void initialize(Subject subject, CallbackHandler handler, Map<String, ?> sharedState, Map<String, ?> options) {}

    @Override
    public boolean login() {
        return true;
    }

    @Override
    public boolean commit() {
        return true;
    }

    @Override
    public boolean abort() {
        return false;
    }

    @Override
    public boolean logout() {
        return true;
    }
}
