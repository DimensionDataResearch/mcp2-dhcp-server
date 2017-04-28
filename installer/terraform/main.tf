/*
 * Deploy a server to supply DHCP / network boot services for a Dimension Data MCP 2.0 network domain
 * **************************************************************************************************
 *
 * Edit the variables below before applying this configuration.
 */

// The MCP region code (AU, NA, etc).
variable "region"               { default = "AU"}

// The Id of the target MCP2.0 datacenter (e.g. NA9, AU10).
variable "datacenter"           { default = "AU9"}

// The name of the target network domain.
variable "networkdomain"        { default = "My Network Domain" }

// The name of the VLAN where DHCP and iPXE services will be provided.
variable "vlan"                 { default = "My VLAN" }

// The public (external) IP of the client machine where Terraform / Ansible are running (used to create firewall rule for SSH).
variable "client_ip"            { default = "1.2.3.4" }

// The name of the server to deploy.
variable "server_name"          { default = "network-services" }

// A private IPv4 address for the server to deploy.
variable "server_ipv4"          { default = "192.168.70.10" }

// The name of the user to install an SSH key for.
variable "ssh_user" { default = "root" }

// The name of a file containing the SSH public key to install on the target server (password authentication will be disabled).
variable "ssh_public_key_file"  { default = "~/.ssh/id_rsa" }

/*
 * Feel free to customise the configuration below if you know what you're doing.
 */
provider "ddcloud" {
    region = "${var.mcp_region}"
}

// Retrieve information about the target network domain.
data "ddcloud_networkdomain" "target_networkdomain" {
    name        = "${var.networkdomain}"
    datacenter  = "${var.datacenter}"
}

// Retrieve information about the target VLAN.
data "ddcloud_vlan" "target_vlan" {
    name            = "${var.vlan}"
    networkdomain   = "${data.ddcloud_networkdomain.target_networkdomain.id}"
}

// Deploy an Ubuntu 16.x server to handle network boot services.
resource "ddcloud_server" "boot_server" {
    name            = "${server_name}"
    description     = "DHCP / network boot services for ${var.vlan}."
    admin_password  = "${var.ssh_bootstrap_password}"

    image = "Ubuntu 16.04 64-bit 2 CPU"

    memory_gb       = 1
    cpu_count       = 1
    cores_per_cpu   = 2

    primary_network_adapter {
        ipv4    = "${var.server_ipv4}"
        vlan    = "${data.dcloud_vlan.target_vlan.id}"
    }
}

// Expose via public IP
resource "ddcloud_nat" "target_server_nat" {
    private_ipv4    = "${ddcloud_server.target_server.primary_network_adapter.ipv4}"
    networkdomain   = "${data.ddcloud_networkdomain.target_networkdomain.id}"
}

// Allow SSH
resource "ddcloud_firewall_rule" "target_server_ssh_inbound" {
    name                = "${server_name}.SSH.Inbound"
    placement           = "first"
    action              = "accept"
    enabled             = true

    ip_version          = "ipv4"
    protocol            = "tcp"

    destination_address = "${ddcloud_nat.target_server_nat.public_ipv4}"
    destination_port    = "22"

    networkdomain       = "${data.ddcloud_networkdomain.target_networkdomain.id}"
}

// Install an SSH key on the target server.
variable "ssh_bootstrap_password" { default = "sn4uSag3s?" sensitive=true }

# Install an SSH key so that Ansible doesn't make us jump through hoops to authenticate.
resource "null_resource" "target_server_ssh_bootstrap" {
    # Install our SSH public key.
	provisioner "remote-exec" {
		inline = [
			"mkdir -p ~/.ssh",
			"chmod 0700 ~/.ssh",
			"echo '${file(var.ssh_public_key_file)}' > ~/.ssh/authorized_keys",
			"chmod 0600 ~/.ssh/authorized_keys",
			"passwd -d root" # Disable password authentication.
		]

		connection {
			type 		= "ssh"
			
			user 		= "${var.ssh_user}"
			password 	= "${var.ssh_bootstrap_password}"

			host 		= "${ddcloud_nat.target_server_nat.public_ipv4}"
		}
	}

    depends_on = [
        "ddcloud_firewall_rule.target_server_ssh_inbound"
    ]
}
