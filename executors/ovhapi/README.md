# Venom - Executor OVHAPI

## Step to test OVH API

Use case: your software needs to make calls to OVH API.<br>
You will need OVH credentials to make API calls. You can either use app keys authentication or OAuth2.

To use app keys authentication, please follow this tutorial: <br>
EN: https://docs.ovh.com/gb/en/customer/first-steps-with-ovh-api/

To use OAuth2, please follow this tutorial: <br>
EN: https://help.ovhcloud.com/csm/en-manage-service-account?id=kb_article_view&sysparm_article=KB0059343

## Input

The following parameters are available:

| Parameter         | Description                                                      | Default Value |
|-------------------|------------------------------------------------------------------|---------------|
| endpoint          | Optional                                                         | ovh-eu        |
| applicationKey    | Optional if `noAuth`, mandatory if using app keys authentication |               |
| applicationSecret | Optional if `noAuth`, mandatory if using app keys authentication |               |
| consumerKey       | Optional if `noAuth`, mandatory if using app keys authentication |               |
| clientID          | Optional if `noAuth`, mandatory if using OAuth2                  |               |
| clientSecret      | Optional if `noAuth`, mandatory if using OAuth2                  |               |
| noAuth            | Optional                                                         |               |
| headers           | Optional                                                         |               |
| resolve           | Optional                                                         |               |
| proxy             | Optional                                                         |               |
| tlsRootCA         | Optional, see [Private root CA](#private-root-ca)                |               |
| tls_root_ca       | Optional, another name for `tlsRootCA`                           |               |

| Parameter | Description | Default Value |
|-----------|-------------|---------------|
| method    | Optional    | GET           |
| path      | Mandatory   |               |
| body      | Optional    |               |
| bodyFile  | Optional    |               |
 
The first batch of parameters can also be defined inside Venom variables like this:

```yaml
vars:
  ovh.endpoint: ovh-eu
  ovh.applicationKey: foo
  ovh.applicationSecret: foo
  ovh.clientID: foo
  ovh.clientSecret: foo
  ovh.consumerKey: foo
  ovh.noAuth: false
  ovh.headers:
    x-foo: foo
  ovh.resolve:
  - example.org:443:127.0.0.1
  ovh.proxy: localhost:8000
  ovh.tlsRootCA: |-
    -----BEGIN CERTIFICATE-----
    MIIF3jCCA8agAwIBAgIQAf1tMPyjylGoG7xkDjUDLTANBgkqhkiG9w0BAQwFADCB
    ...
    -----END CERTIFICATE-----
```

### Private root CA

`tlsRootCA` and `tls_root_ca` are two names for one setting: a CA the executor trusts on top of the system's,
for an API behind a private certificate. Set one of them; a step that sets both fails before any call.
The `ovh.tlsRootCA` variable, the one variable for both names, is used only when the step sets neither.

The value is the PEM text of the certificate, or the path of a PEM file. A relative path is read from the
folder of the test suite. The `ovh.tlsRootCA` variable takes a path too.

The root CA only serves TLS connections. With an `http://` endpoint the step logs a warning; a redirect to
`https://` or an `https://` proxy still uses the CA. An endpoint name such as `ovh-eu` is an `https://` URL and does not warn.

```yaml
- type: ovhapi
  endpoint: 'https://api.example.internal/1.0'
  tls_root_ca: certs/internal-ca.pem
  noAuth: true
  path: /me
```

## Example of an __ovhapi__ TestSuite

### Using App Keys authentication

```yaml
name: Title of TestSuite
testcases:
- name: me
  steps:
  - type: ovhapi
    endpoint: 'ovh-eu'
    applicationKey: 'APPLICATION_KEY'
    applicationSecret: 'APPLICATION_SECRET'
    consumerKey: 'CONSUMER_KEY'
    method: GET
    path: /me
    retry: 3
    delay: 2
    assertions:
    - result.statuscode ShouldEqual 200
    - result.bodyjson.nichandle ShouldContainSubstring MY_NICHANDLE
```

### Using OAuth2

```yaml
name: Title of TestSuite
testcases:
- name: me
  steps:
  - type: ovhapi
    endpoint: 'ovh-eu'
    clientID: 'CLIENT_ID'
    clientSecret: 'CLIENT_SECRET'
    method: GET
    path: /me
    retry: 3
    delay: 2
    assertions:
    - result.statuscode ShouldEqual 200
    - result.bodyjson.nichandle ShouldContainSubstring MY_NICHANDLE
```

## Output

The following output fields are available:

| Field              | Description                          |
|--------------------|--------------------------------------|
| result.executor    |                                      |
| result.timeseconds | Execution duration                   |
| result.statuscode  | Status Code of HTTP response         |
| result.body        | Body of HTTP response                |
| result.bodyjson    | Body of HTTP response if it's a JSON |
| result.err         | Error message if exists              |

Note that you can access json data as `result.bodyjson.yourkey` for example.

## Default assertion

```yaml
result.statuscode ShouldEqual 200
```
