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
  vlan_id: "MyVLAN"
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

* `boot_image` is the name of the initial iPXE boot image file sent to regular PXE clients.  
PXE clients will load this image via TFTP (from the server where `mcp2-dhcp-server` is running).  
When they load this image, iPXE will send a second discovery packet with a user class of `iPXE`.
* `boot_script` is the URL of the iPXE script (HTTP or TFTP) sent to iPXE clients.

If you're trying to boot CoreOS, consider using [coreos-ipxe-server](https://github.com/kelseyhightower/coreos-ipxe-server).

## Network boot in MCP2

1. Create a VM using VMWare Workstation or VMWare fusion (ensure it has 1 disk and 1 network adapter, with hardware versiom <= 10).
2. Do not install an operating system (leave the disk completely empty).
3. Close VMWare, and use [ovftool](https://my.vmware.com/web/vmware/details?downloadGroup=OVFTOOL400&productId=353) to convert the virtual machine to OVF format (`ovftool myserver.vmx ovf/myserver.ovf`).
4. Upload the `.ovf`, `.vmdk`, and `.mf` files to CloudControl and import it as a client image (ensure that "Import without Guest OS Customization" is checked, see [here](https://docs.mcp-services.net/display/CCD/How+to+Import+an+OVF+Package+as+a+Client+Image) for details).
5. Run `mcp2-dhcp-server` on the same machine as your TFTP / iPXE server.
6. Deploy a new server from your client image, and start it. Network boot should proceed automatically.

