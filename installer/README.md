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
8. `ansible all -m ping` (ensure you get a valid response)
9. Update variables in `group_vars/net-boot-service.yml`.
10. `ansible-playbook install-mcp2-dhcp-server`

