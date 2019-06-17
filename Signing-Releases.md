# Signing Releases

We use OpenSSL to sign releases, ensuring their integrity and origin.

## Generate a signing key

On the developer's machine, generate a public and private key pair:

````sh
openssl genrsa \
  -out codesign.key \
  4096
openssl rsa \
  -in codesign.key \
  -pubout \
  -out codesign.pub
````

## Sign a file

Example of signing a file, with a base64-encoded signature:

````sh
FILE="file-to-sign.txt"

openssl dgst \
  -sha256 \
  -sign codesign.key \
  $FILE \
| base64 > $FILE.sig
````

## Verify a signature

Example of verifying a signature:

````sh
FILE="file-to-sign.txt"

base64 -d $FILE.sig > $FILE.sig.bin
openssl dgst \
  -sha256 \
  -verify codesign.pub \
  -signature $FILE.sig.bin $FILE
````
