Gigamon FM Terraform Provider
-----------------------------

This provides a Terraform provider for Gigamon FM Cloud solutions. 

Installation
------------
Currently this is not being hosted on any external TF repository. Users will have to have this in
their local system

Please copy the terraform binary "terraform-provider-gigamon" to the following directory in your
system

~/.terraform.d/plugins/local/gigamon/gigamon/1.0.0/linux_amd64/terraform-provider-gigamon

Note: That we only support linux-amd64 binary now, and if we wamt MAC or other OS than we need
to build the binary for those systems

Installation and Testing For developers
----------------------------------------
  - Create the following in your home directory
    .terraform.d/plugins/local/gigamon/gigamon/1.0.0/linux_amd64/

  - Please setup the environment variable GOBIN to the following
    <your home>/.terraform.d/plugins/local/gigamon/gigamon/1.0.0/linux_amd64/

  - After any changes to the source do a go install, which will build the updated binary and
    copy it to the above directory automatically. That will ensure that your TF will run with
    the latest changes

Generating Docs
----------------
Run tfplugindocs from the base directory, and it will produce the markdown files under the doc
  directory

Run the convert_md_html.py and it will traverse the doc directory and convert all the .md files
  to the corresponding html files

Copy these to the /var/www directory and will get rendered properly.

Examples and Usages
-------------------

Current Supported Features
--------------------------

Future Support Planned
----------------------

