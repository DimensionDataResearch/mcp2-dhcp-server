docker run -it local/mcp2build4rancheros -e MCP_USER=ccallanan_eng \
-e MCP_PASSWORD=C@ll01974 \
-e MCP_REGIOn=au \
-e MCP_VLAN_ID=3250eb64-8037-43f0-b04f-2d1de1e5cc4b \
-e DHCP_SERVICE_IP=10.2.2.10 \
-e RSA_KEY=ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCmNeV/W/BNuxlxJCtSmmZ+5Cg4R7qA+H0ndKEci8bke8pbxwrkga4Tmmed5bH/uaHe1s4eqn/eECHjIXmfKWyIQmQxIdjz7Z4NIUPhGRfDvvLIosVfy9WLutiDjXfo7mqeC8ctQ5bBU6VYQaXtgLSAL6KjrKOgOVqmnF0wdCtz+4VkKYGbL04vG7YqFeEGHozasOeQW3zB4a8BIvZHwSeM9bhXOqnQMdrQ5w3uggCgKB3QX6bkjThRwwpivqB6Er/jKReBTsx1qwDLLNi03VJl1vqTMpqfhq1zQJIDf2I9S+5w4sP4wfSu6EAp3sJo2a18Nmd4K18GujNxVDzKTRDf \
-e RANCHEROS_PASSWORD=DevAdmin123 \
-e RANCHER_AGENT_VERSION=rancher/agent:v1.2.2 \
-e RANCHER_AGENT_URL=https://rancherlab.itaas-cloud.com/v1/scripts/00873662D526E62B1F97:1483142400000:k7wZDCXQmgC0kIWkJmjSAldWeqc
