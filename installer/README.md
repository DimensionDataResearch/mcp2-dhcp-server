# Automated installation

Requires:

* Ansible v2.2 or higher
* Terraform v0.9 or higher
* [terraform-provider-ddcloud](https://github.com/DimensionDataResearch/dd-cloud-compute-terraform/releases/download/v1.3.0-alpha3/terraform-provider-ddcloud.v1.3.0-alpha3.linux-amd64.zip)

1. Update variables in `terraform/main.tf`.
2. Place username and password in `MCP_USERNAME` / `MCP_PASSWORD` environment variables.
3. `cd terraform`
4. `terraform plan` (verify that you're happy with the plan)
5. `terraform apply`
6. `terraform refresh`
7. `cd ..`
Change hostfile=./terraform/environments/base/terraform_inventory.py to the terraform environment you created above
8. `ansible dhcp_boot_server -m ping` (ensure you get a valid response if not check firewall rules)
9. Update variables in `group_vars/net-boot-service.yml`.
<<<<<<< HEAD
<<<<<<< HEAD
10. `ansible-playbook install-mcp2-dhcp-server-rancher.yml --tags "common,coreos,rancheros,rancherosnfs` (Note remove the tags you don't want i.e. remove coreos if you want rancheros and remove rancherosnfs if you dont want to use nfs)
11. Log into the web UI and start the rancher vms that were created. The rancheros vms will be built from the pxe build environment and they should autobuild and appear in your rancher environemnt
=======
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
10. `ansible-playbook install-mcp2-dhcp-server-rancher.yml --tags "common,coreos,rancheros,rancherosnfs` (Note remove the tags you don't want i.e. remove coreos if you want rancheros and remove rancherosnfs if you dont want to use nfs)
11. Log into the web UI and start the rancher vms that were created. The rancheros vms will be built from the pxe build environment and they should autobuild and appear in your rancher environemnt
=======
=======
>>>>>>> 82fd5496769a5c56fcf4117aa39061bf8b4b3a90
=======
>>>>>>> 82fd5496769a5c56fcf4117aa39061bf8b4b3a90
10. `ansible-playbook install-mcp2-dhcp-server`
11. When completed boot the rancheros vms and they should autobuild and appear in your rancher environemnt
>>>>>>> parent of 31fa745... Updates to include NFS on the DHCP server as an option
=======
10. `ansible-playbook install-mcp2-dhcp-server`
11. When completed boot the rancheros vms and they should autobuild and appear in your rancher environemnt
>>>>>>> parent of 31fa745... Updates to include NFS on the DHCP server as an option
<<<<<<< HEAD
<<<<<<< HEAD
>>>>>>> 82fd5496769a5c56fcf4117aa39061bf8b4b3a90
=======
10. `ansible-playbook install-mcp2-dhcp-server`
11. When completed boot the rancheros vms and they should autobuild and appear in your rancher environemnt
>>>>>>> parent of 31fa745... Updates to include NFS on the DHCP server as an option
=======
>>>>>>> 82fd5496769a5c56fcf4117aa39061bf8b4b3a90
=======
>>>>>>> 82fd5496769a5c56fcf4117aa39061bf8b4b3a90

