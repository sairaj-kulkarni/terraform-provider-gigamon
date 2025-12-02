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

  - After any changes to the source
      go to the base directory of the repo i.e. to "fm_terraform_provider"
      execute go install ./terraform-provider
      This will generate the binary and also install it in the directory pointed to by GOBIN

  - Using the new version and testing in TF modules
     Currently we are not yet versioning the module, so every build will overwrite the same
       version
    after doing the above go install, than go to the TF directory (where you have your main.tf
      or other files)
    rm the .terraform directory and .terraform.lock.hcl (this is because they will have the 
       checksum of the previous build and will not match the current build). We will fix this
       by either introducing versioning or by ensuring that we upload the same checksum for
       every build
    do a terraform init (which will download the module again and in our case just get from
        local)
    then proceed with terraform plan or terraform apply etc. as required.

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

Bulding a website for static navigation using left sider bar 

https://www.w3schools.com/howto/howto_css_fixed_sidebar.asp
