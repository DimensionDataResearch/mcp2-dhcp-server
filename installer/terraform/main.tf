/*
 * Deploy a server to supply DHCP / network boot services for a Dimension Data MCP 2.0 network domain
 * **************************************************************************************************
 *
 * Edit the variables below before applying this configuration.
 *
 * Place username and password in MCP_USERNAME / MCP_PASSWORD environment variables.
 */

// The MCP region code (AU, NA, etc).
variable "mcp_region"           { default = "AU"}

// The Id of the target MCP2.0 datacenter (e.g. NA9, AU10).
variable "datacenter"           { default = "AU9"}

// The name of the target network domain.
variable "networkdomain"        { default = "My Network Domain" }

// The name of the VLAN that the server will be attached to.
variable "vlan"                 { default = "My VLAN" }

// The public (external) IP of the client machine where Terraform / Ansible are running (used to create firewall rule for SSH).
variable "client_ip"            { default = "1.2.3.4" }

// The name of the server to deploy.
variable "server_name"          { default = "network-services" }

// The primary IPv4 address for the server to deploy.
variable "server_ipv4"          { default = "192.168.70.10" }

// The name of the user to install an SSH key for.
variable "ssh_user" { default = "root" }

// The name of a file containing the SSH public key to install on the target server (password authentication will be disabled).
variable "ssh_public_key_file"  { default = "~/.ssh/id_rsa.pub" }

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

// Retrieve information about the target VLANs.
data "ddcloud_vlan" "target_vlan" {
    name            = "${var.vlan}"
    networkdomain   = "${data.ddcloud_networkdomain.target_networkdomain.id}"
}

// Deploy an Ubuntu 16.x server to handle network boot services.
resource "ddcloud_server" "target_server" {
    name            = "${var.server_name}"
    description     = "DHCP / network boot services for ${var.vlan}."
    admin_password  = "${var.ssh_bootstrap_password}"
    auto_start      = true

    image = "Ubuntu 16.04 64-bit 2 CPU"

    memory_gb       = 2
    cpu_count       = 2

    networkdomain   = "${data.ddcloud_networkdomain.target_networkdomain.id}"

    primary_network_adapter {
        ipv4    = "${var.server_ipv4}"
        vlan    = "${data.ddcloud_vlan.target_vlan.id}"
    }

    tag {
        name    = "roles"
        value   = "net-boot-service"
    }
}

// Expose via public IP
resource "ddcloud_nat" "target_server_nat" {
    private_ipv4    = "${ddcloud_server.target_server.primary_network_adapter.0.ipv4}"
    networkdomain   = "${data.ddcloud_networkdomain.target_networkdomain.id}"
}

// Allow SSH
resource "ddcloud_address_list" "clients" {
    name            = "Clients"
    ip_version      = "IPv4"
    
    addresses = [
        "${var.client_ip}"
    ]

    networkdomain   = "${data.ddcloud_networkdomain.target_networkdomain.id}"
}
resource "ddcloud_firewall_rule" "target_server_ssh_inbound" {
    name                = "${var.server_name}.SSH.Inbound"
    placement           = "first"
    action              = "accept"
    enabled             = true

    ip_version          = "ipv4"
    protocol            = "tcp"

    source_address_list = "${ddcloud_address_list.clients.id}"

    destination_address = "${ddcloud_nat.target_server_nat.public_ipv4}"
    destination_port    = "22"

    networkdomain       = "${data.ddcloud_networkdomain.target_networkdomain.id}"
}

// Install an SSH key on the target server.
variable "ssh_bootstrap_password" { default = "sn4uSag3s?" }

# Perform initial provisioning, including installation of an SSH key (so that Ansible doesn't make us jump through hoops to authenticate).
resource "null_resource" "target_server_provision" {
    # Install our SSH public key.
	provisioner "remote-exec" {
		inline = [
            # Install SSH key
			"mkdir -p ~/.ssh",
			"chmod 0700 ~/.ssh",
			"echo '${file(var.ssh_public_key_file)}' > ~/.ssh/authorized_keys",
			"chmod 0600 ~/.ssh/authorized_keys",
			
            # Disable password authentication.
            "passwd -d root",
            
            # Ensure that python is installed (required by Ansible).
            "apt-get install -y python",

            # Configure default locale
            "apt-get install -y language-pack-en"
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

output "server_name" {
    value = "${ddcloud_server.target_server.name}"
}
output "server_public_ipv4" {
    value = "${ddcloud_nat.target_server_nat.public_ipv4}"
}
output "server_private_ipv4" {
    value = "${ddcloud_server.target_server.primary_network_adapter.0.ipv4}"
}
output "server_vlan_id" {
    value = "${ddcloud_server.target_server.primary_network_adapter.0.vlan}"
}
