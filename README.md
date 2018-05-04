# mcp2-dhcp-server
A DHCP / DNS / PXE / iPXE server driven server driven by MCP 2.0 server metadata (from Dimension Data CloudControl).

The primary purpose of this service is to enable you (via PXE / iPXE) to boot operating systems that require the use of cloud-init (e.g. RancherOS, CoreOS / Container Linux). It can also be configured to provide only simple DHCP / DNS facilities if PXE / iPXE is not required.

## Configuration
Create `mcp2-dhcp-server.yml`:

```yaml
mcp:
  user: "my_mcp_user"
  password: "my_mpc_password"
  region: "AU"

network:
  # Specify the interface to listen on (for now, only a single interface is supported).
  interface: eth0
  vlan_id: "42837f37-a0fd-4544-a800-416a1d33f672"
  service_ip: 192.168.70.12
```

## DNS
The service can also answer DNS queries for a pseudo-zone whose records come from server metadata in CloudControl.
It can answer queries for the following record types:

* `A` (name -> IPv4 address)
* `AAAA` (name -> IPv6 address)
* `PTR` (IPv4 / IPv6 address -> name)

All other query types (and `PTR` queries that cannot be answered locally) will be forwarded to the fallback server.

If you want to enable DNS, add the following to `mcp2-dhcp-server.yml`:

```yaml
dns:
  enable: true

  # The port to listen on.
  port: 53
  
  # The suffix for the pseudo-zone containing MCP servers
  # For example, if your server is named "server1", then this can be resolved as "server1.my-environment.mcp".
  #
  # Any suffix will do, but preferably one that's not a real domain name.
  domain_name: my-environment.mcp

  # The time-to-live (TTL), in seconds, for records in the the pseudo-zone containing MCP servers.
  default_ttl: 60
  
  # This is the fallback DNS server; any queries that cannot be answered locally will be forwarded to 
  forwarding:
    to_address: 8.8.8.8
    to_port: 53
```

The values above (apart from `enable`) are the default values and can be omitted unless they differ.

Note that (for now) the service will only listen for DNS queries on the first IP address assigned to the network interface defined above in the `network` section.

## PXE / iPXE
If you're using iPXE, add the following to `mcp2-dhcp-server.yml`:

```yaml
ipxe:
  enable: true
  port: 4777 # The TCP port used by the IPXE server (e.g. coreos-ixpe-server). Usually matches boot_script below.
  boot_image: "undionly.kpxe"
  boot_script: "http://192.168.220.10:4777/?profile=development"
```

* `boot_image` is the name of the initial iPXE boot image file (relative to `/var/lib/tftpboot`) sent to regular PXE clients.  
PXE clients will load this image via TFTP (from the server where `mcp2-dhcp-server` is running).  
When they load this image, iPXE will send a second discovery packet with a user class of `iPXE`.
* `boot_script` is the URL of the iPXE script (HTTP or TFTP) sent to iPXE clients.

If you're trying to boot CoreOS, consider using [coreos-ipxe-server](https://github.com/kelseyhightower/coreos-ipxe-server).

### Overriding configuration with server tags

You can customise PXE / iPXE behaviour in CloudControl by giving a server one or more of the following tags:

* `pxe_boot_image` (optional) - if specified, overrides the name of the initial PXE boot image to use (relative to `/var/lib/tftpboot` on the TFTP server).
* `ipxe_profile` (optional) - if specified, overrides the name of the iPXE profile to use (equivalent to specifying `ipxe_boot_script` = `http://{network.service_ip}:{ipxe.port}:4777/?profile={ipxe_profile}`).
* `ipxe_boot_script` (optional) - if specified, overrides the URL of the iPXE boot script to use (also overrides `ipxe_profile`).

## Installation

See [here](installer/README.md) for instructions.

### Prerequisites

* Ansible v2.2 or higher
* [Terraform](https://terraform.io/) v0.9.x or higher
* [terraform-provider-ddcloud](https://github.com/DimensionDataResearch/dd-cloud-compute-terraform/releases/download/v1.3.0-alpha3/terraform-provider-ddcloud.v1.3.0-alpha3.linux-amd64.zip)

### Net-bootable image

1. Download the image files [here](https://ddcbu.blob.core.windows.net/public/mcp/net-boot.zip) and unzip them.
2. Upload the `.ovf`, `.vmdk`, and `.mf` files to CloudControl and import them as a client image (ensure that "Import without Guest OS Customization" is checked, see [here](https://docs.mcp-services.net/display/CCD/How+to+Import+an+OVF+Package+as+a+Client+Image) for details).

### Net-bootable image (create your own)

1. Create a VM using VMWare Workstation or VMWare fusion (ensure it has 1 disk and 1 network adapter, with hardware version <= 10).
2. Do not install an operating system (leave the disk completely empty).
3. Close VMWare, and use [ovftool](https://my.vmware.com/web/vmware/details?downloadGroup=OVFTOOL400&productId=353) to convert the virtual machine to OVF format (`ovftool myserver.vmx ovf/myserver.ovf`).
4. Upload the `.ovf`, `.vmdk`, and `.mf` files to CloudControl and import them as a client image (ensure that "Import without Guest OS Customization" is checked, see [here](https://docs.mcp-services.net/display/CCD/How+to+Import+an+OVF+Package+as+a+Client+Image) for details).

### Putting it all together

Deploy a new server from your client image, and start it. Network boot should proceed automatically.
