For hosting TF private registry, we need to prepare the release files.
We need to sign the shasum fil using GPG, and hence we need a public and private gpg key pair
given a binary, along with the metadata (like the version, the os and arch) that this binary
is built for, we will use the prepare_release script to generate the necessary files and
will host all of them in the release directory.

For help in generating gpg keys and signing files with them, see the link
https://www.google.com/search?q=linux+how+to+sign+a+file+with+gpg+keys&rlz=1C5GCEM_en&oq=linux+how+to+sign+a+file+with+gpg+keys&gs_lcrp=EgZjaHJvbWUyBggAEEUYOTIJCAEQIRgKGKABMgcIAhAhGJ8FMgcIAxAhGI8CMgcIBBAhGI8C0gEINjgzOGowajGoAgiwAgHxBXvXcJyG1jmN&sourceid=chrome&ie=UTF-8

As a one time, setup we would have to run the setup_gpg.sh, which will
1. Create the gpg pubic and private key and store it in the local directory gpg_keys
2. Get and store the public key-id and the public key copy in files in the local
    direcotry 
	The public key-id and the public key in armor format will be sent to TF later 
	when the terraform init command is run

For details on the TF registry protocol, plese see the link
https://developer.hashicorp.com/terraform/internals/provider-registry-protocol

Steps to generate a build and host it on the local registry

1. Generate the build, and keep it in the builds directory, with the name as 
    terraform-provider-gigamon_{version}_{os_type}_{arch_type}
	where version is the version of the provider for e.g. 6.14.00, os_type will be linux for
	now and the arch will be amd64 for now
2. Run the command build.py with the appropriate parameters and it will generate the necessary
   artifacts in the artifact directory

