# CA

This folder contains a TLS certificate and a certification authority that is used for the development of Statiko.

This certification authority is unsafe and you should not add it to your CA trust store. The private key for the CA is purposedly included in this repository to signal that the CA is compromised.

The JSON files are used by cfssl to conveniently generate the certificates (and re-generate them if needed).
