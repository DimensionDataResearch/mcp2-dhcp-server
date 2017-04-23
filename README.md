# mcp2-dhcp-server
A DHCP server driven by MCP 2.0 server metadata (from Dimension Data CloudControl)

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
* `boot_script` is the URL of the iPXE script sent to iPXE clients.
