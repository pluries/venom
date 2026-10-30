# Venom - Executor SMTP

Step for sending SMTP

Use case: your software has to check mails to do something with them?
Venom can send mail then execute some tests on your software.

## Input

```yaml
name: TestSuite with SMTP Steps
testcases:
- name: TestCase SMTP
  steps:
  - type: smtp
    withtls: false
    verifytls: false # Optional, used with withtls: true (see TLS below)
    host: localhost
    port: 25 # 465 if using TLS
    user: yourSMTPUsername # Optional, only works with TLS
    password: yourSMTPPassword # Optional, only works with TLS
    from: venom@smtp.net
    to: destinationa@yourdomain.com,destinationb@yourdomain.com
    subject: Title of mail
    body: Body of mail
    tls_root_ca: ca.pem # Optional, used with withtls: true and verifytls: true (see TLS below)
    tls_client_cert: client.pem # Optional, used with withtls: true, together with tls_client_key
    tls_client_key: client.key # Optional, used with withtls: true, together with tls_client_cert
    assertions:
    - result.err ShouldBeEmpty
```

### TLS

| Parameter | Description | Default Value |
|---|---|---|
| withtls | Optional. Connect over TLS from the first byte (usually port 465) | false |
| verifytls | Optional, used with `withtls: true`. `false`: the server certificate is not checked, so a self-signed test server is accepted. `true`: the certificate must chain to a system root or to `tls_root_ca` and be valid for `host`, or the step fails | false |
| tls_root_ca | Optional, used with `withtls: true`. CA certificates trusted on top of the system roots, so that `verifytls: true` accepts a server signed by a private CA. Several CAs can be concatenated. It is used to verify only when `verifytls: true` | |
| tls_client_cert | Optional, used with `withtls: true`. The client certificate (mutual TLS), the leaf first, followed by intermediates. It is sent even when `verifytls` is `false`. It needs `tls_client_key`: a certificate without its key fails the step | |
| tls_client_key | Optional, used with `withtls: true`. The private key of `tls_client_cert`. It needs `tls_client_cert`: a key without its certificate fails the step | |

`tls_root_ca`, `tls_client_cert` and `tls_client_key` take the PEM content itself or the path to a PEM file, relative
to the test suite's directory. Without `withtls: true` they are not used: the connection stays plaintext and a warning
is logged (a certificate without its key, or the reverse, still fails the step).

The `SSL_CERT_FILE` or `SSL_CERT_DIR` environment variable can point venom at another set of system roots.

## Output

Nothing, except result.err if there is an error.

## Default assertion

```yaml
result.err ShouldBeEmpty
```
