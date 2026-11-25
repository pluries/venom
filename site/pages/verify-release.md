Every release of this fork is signed, and you can check the signature without trusting the download page. This page shows how to prove that a release is the maintainer's: that the checksum list is signed, that the signer is the maintainer's certificate, and that your file is on the list. You need `openssl` 3 or later, `sha256sum` and `curl`. The LibreSSL that macOS ships cannot check the timestamp of the signature, so install OpenSSL 3 there first.

Download the checksum list, its signature and the executable you want:

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ curl -LO https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
```

## Check the signer

Extract the certificate that signed the list and print its SHA-256 fingerprint:

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -noverify -signer signer.pem -out /dev/null
$ openssl x509 -in signer.pem -outform DER | sha256sum
```

The fingerprint of the signer of every release from `v1.5.0-pluries.2` on is:

```
42632819c286ec58a829fea06323c5e53772982d7eaff3d2756bd997527a54f2
```

If your result differs, stop: the list was not signed by the maintainer's certificate.

## Check the list and the file

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
$ sha256sum -c SHA256SUMS --ignore-missing
```

The first command proves that the list was signed and not changed. The second proves that your executable is the one on the list.

## Let the venom-pluries skill do it

The `venom-pluries` skill for Claude Code carries a script that runs all of these checks, compares the signer with the fingerprint above and refuses to install anything that fails. Use it when you add venom to a project.
