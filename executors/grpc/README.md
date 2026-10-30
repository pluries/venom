# Venom - Executor Grpc

Step to execute a gRPC request

Based on `grpcurl`, see [grpcurl](https://github.com/fullstorydev/grpcurl) for more information.
This executor relies on the gRPC server reflection, which should be enabled on the server as described 
[here](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md). 
gRPC server reflection is not enabled by default and not implemented for every gRPC library,
make sure your library of choice supports reflection before implementing tests using this executor.
gRPC server reflection also does not properly work with `gogo/protobuf`: grpc/grpc-go#1873

## Tests

Results of tests are parsed as JSON and saved in `systemoutjson`. Status codes correspond 
to the official status codes of gRPC.
You can find what individual return codes mean [here](https://github.com/grpc/grpc/blob/master/doc/statuscodes.md).

## Input

In your yaml file, you can use:

```yaml
  - url mandatory
  - service mandatory: service to call
  - method mandatory: list, describe, or method of the endpoint
  - data optional: data to marshal to JSON and send as a request
  - headers optional: data to send as additional headers
  - connect_timeout optional: The maximum time, in seconds, to wait for connection to be established. Defaults to 10 seconds
  - default_fields optional: whether json formatter should emit default fields
  - include_text_separator optional: when protobuf string formatter is invoked to format multiple messages, all messages after the first one will be prefixed with character (0x1E)
  - tls_client_cert optional: a chain of certificates to identify the caller, first certificate in the chain is considered as the leaf, followed by intermediates. Setting it enables mutual TLS authentication, and the certificate is sent even when ignore_verify_ssl is true. Set the PEM content or the path to the PEM file. It needs tls_client_key: a certificate without its key fails the step. It is not used without tls_root_ca: the connection stays plaintext and a warning is logged.
  - tls_client_key optional: private key corresponding to the certificate. Set the PEM content or the path to the PEM file. It needs tls_client_cert: a key without its certificate fails the step. It is not used without tls_root_ca: the connection stays plaintext and a warning is logged.
  - tls_root_ca optional: root CAs trusted to verify the server certificate, on top of the system ones. Can contain multiple CAs concatenated together. Set the PEM content or the path to the PEM file. The step uses TLS only when this is set; without it the connection is plaintext.
  - ignore_verify_ssl optional: set to true to skip the server certificate check, for example a self-signed certificate that tls_root_ca does not cover
```

Example:

```yaml

name: Title of TestSuite
testcases:

- name: request GRPC
  steps:
  - type: grpc
    url: serverUrlWithoutHttp:8090
    data:
      foo: bar
    service: coolService.api
    method: GetAllFoos
    assertions:
    - result.code ShouldEqual 0
    - result.systemoutjson.foo ShouldEqual bar
```

Example TLS:

```yaml

name: Title of TestSuite
testcases:

- name: request GRPC
  steps:
  - type: grpc
    url: serverUrlWithoutHttp:8090
    tls_root_ca: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    # ignore_verify_ssl: true # only when tls_root_ca and the system CAs do not cover the server certificate
    data:
      foo: bar
    service: coolService.api
    method: GetAllFoos
    assertions:
    - result.code ShouldEqual 0
    - result.systemoutjson.foo ShouldEqual bar
```

Example mutual TLS:

```yaml

name: Title of TestSuite
testcases:

- name: request GRPC
  steps:
  - type: grpc
    url: serverUrlWithoutHttp:8090
    tls_root_ca: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    tls_client_cert: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    tls_client_key: |-
      -----BEGIN PRIVATE KEY-----
      ...
      -----END PRIVATE KEY-----
    # ignore_verify_ssl: true # only when tls_root_ca and the system CAs do not cover the server certificate
    data:
      foo: bar
    service: coolService.api
    method: GetAllFoos
    assertions:
    - result.code ShouldEqual 0
    - result.systemoutjson.foo ShouldEqual bar
```

## Output

```yaml
executor
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds: execution duration
- result.executor.executor.script: script executed
- result.err: if exists, this field contains error
- result.systemout: Standard Output of executed script
- result.systemerr: Error Output of executed script
- result.code: Exit Code
