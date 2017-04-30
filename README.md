# mcp2-dhcp-server
A DHCP server driven by MCP 2.0 server metadata (from Dimension Data CloudControl)

**Note**: This is a work-in-progress and has received only limited testing. It is not production-ready yet.

## Configuration
Create `mcp2-dhcp-server.yml`:

```yaml
mcp:
  user: "my_mcp_user"
  password: "my_mpc_password"
  region: "AU"

network:
  interface: eth0 # Specify the interface to listen on (for now, only a single interface is supported).
  vlan_id: "42837f37-a0fd-4544-a800-416a1d33f672"
  service_ip: 192.168.70.12
  start_ip: 192.168.70.20
  end_ip: 192.168.70.30
```

## PXE / iPXE
If you're using iPXE, add the following to `mcp2-dhcp-server.yml`:

```yaml
ipxe:
  enable: true
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
* `ipxe_profile` (optional) - if specified, overrides the name of the iPXE profile to use (equivalent to specifying `ipxe_boot_script` = `http://service_ip/?profile=ipxe_profile`).
* `ipxe_boot_script` (optional) - if specified, overrides the URL of the iPXE boot script to use (also overrides `ipxe_profile`).

## Network boot in MCP2

### Prerequisites

#### DHCP / TFTP / iPXE server

Deploy an Ubuntu 16.x server attached to the target VLAN (other distros might work but have not been tested):

1. `apt-get install -y git build-essential liblzma-dev mkisofs tftpd-hpa`.
2. `mkdir -p /usr/local/src/ipxe`
3. `cd /usr/local/src/ipxe`
4. `git clone git://git.ipxe.org/ipxe.git .`
5. `cd src`
6. `make`
7. `cp bin/undionly.kpxe /var/lib/tftpboot`
8. Place a copy of the `mcp2-dhcp-server` [executable](https://github.com/DimensionDataResearch/mcp2-dhcp-server/releases/download/v0.1-alpha1/mcp2-dhcp-server) on this machine.

If you're using `coreos-ipxe-server`:

1. Place a copy of the `coreos-ipxe-server` [executable](https://github.com/kelseyhightower/coreos-ipxe-server/releases/download/v0.3.0/coreos-ipxe-server-0.3.0-linux-amd64) on this machine.
2. `export COREOS_IPXE_SERVER_DATA_DIR=/opt/coreos-ipxe-server`
3. `mkdir -p $COREOS_IPXE_SERVER_DATA_DIR/{configs,images,profiles,sshkeys}`
4. `mkdir -p $COREOS_IPXE_SERVER_DATA_DIR/images/amd64-usr/310.1.0`
5. `pushd $COREOS_IPXE_SERVER_DATA_DIR/images/amd64-usr/310.1.0`
6. `wget http://storage.core-os.net/coreos/amd64-usr/310.1.0/coreos_production_pxe_image.cpio.gz`
7. `wget http://storage.core-os.net/coreos/amd64-usr/310.1.0/coreos_production_pxe.vmlinuz`
8. `popd`
9. Place an SSH public key in `$COREOS_IPXE_SERVER_DATA_DIR/sshkeys/coreos.pub`
10. Place a cloud-config in `$COREOS_IPXE_SERVER_DATA_DIR/configs/development.yml`:  
```yaml
#cloud-config

ssh_authorized_keys:
    - ssh-rsa AAAAB3Nza... # Place your real SSH key here, or remove this section.
coreos:
  etcd:
    addr: $private_ipv4:4001
    peer-addr: $private_ipv4:7001
  units:
    - name: etcd.service
      command: start
    - name: fleet.service
      command: start
    - name: docker.socket
      command: start
  oem:
    id: coreos
    name: CoreOS Custom
    version-id: 310.1.0
    home-url: https://coreos.com
```
11. Place a profile in `$COREOS_IPXE_SERVER_DATA_DIR/profiles/development.json`:  
```json
{
	"cloud_config": "development",
	"rootfstype": "btrfs",
	"sshkey": "coreos",
	"version": "310.1.0"
}
```

### Process

1. Create a VM using VMWare Workstation or VMWare fusion (ensure it has 1 disk and 1 network adapter, with hardware version <= 10).
2. Do not install an operating system (leave the disk completely empty).
3. Close VMWare, and use [ovftool](https://my.vmware.com/web/vmware/details?downloadGroup=OVFTOOL400&productId=353) to convert the virtual machine to OVF format (`ovftool myserver.vmx ovf/myserver.ovf`).
4. Upload the `.ovf`, `.vmdk`, and `.mf` files to CloudControl and import it as a client image (ensure that "Import without Guest OS Customization" is checked, see [here](https://docs.mcp-services.net/display/CCD/How+to+Import+an+OVF+Package+as+a+Client+Image) for details).
5. On your DHCP / iPXE server, run `coreos-ipxe-server`.
6. On your DHCP / iPXE server, run `mcp2-dhcp-server`.
7. Deploy a new server from your client image, and start it. Network boot should proceed automatically.
