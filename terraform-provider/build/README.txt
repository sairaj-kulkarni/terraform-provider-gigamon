For hosting TF private registry, we need to prepare the release files.
We need to sign the shasum fil using GPG, and hence we need a public and private gpg key pair
given a binary, along with the metadata (like the version, the os and arch) that this binary
is built for, we will use the prepare_release script to generate the necessary files and
will host all of them in the release directory.

For help in generating gpg keys and signing files with them, see the link
https://www.google.com/search?q=linux+how+to+sign+a+file+with+gpg+keys&rlz=1C5GCEM_en&oq=linux+how+to+sign+a+file+with+gpg+keys&gs_lcrp=EgZjaHJvbWUyBggAEEUYOTIJCAEQIRgKGKABMgcIAhAhGJ8FMgcIAxAhGI8CMgcIBBAhGI8C0gEINjgzOGowajGoAgiwAgHxBXvXcJyG1jmN&sourceid=chrome&ie=UTF-8

Key Details:
Key ID: E59CE99291948BBB
armored Public key (i.e. pubic key in ascii format) - tf_public_key.asc
The private/public key found in ~/.gnupg
The pass phrase of the private key is jana1234
